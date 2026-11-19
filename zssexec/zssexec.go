// Package zssexec executes character/zss's parsed Statedef/Function
// script bodies against a fighter's live evaluator.Context -- the .zss
// counterpart to engine/statemachine's Step for .cns-driven state
// execution.
//
// The grammar mirrors Ikemen GO's .zss compiler; see parser.go,
// .vibe/decisions/008 and 022. A body is read as one continuous text, so
// statements may span lines. Supported statements:
//
//   - [ignoreHitPause] [persistent(N)] if COND { ... } [else if ... | else { ... }]
//     and the same attributes before a bare { ... } block
//   - Name{key: value; ...}        -- a controller call, applied through
//     statemachine.ApplyController
//   - let a, b = e1, e2;  /  let a, b = call F(args);  -- locals, read as $a
//   - call F(args);                -- calls a Function block
//   - EXPR;                        -- an expression with an assignment, e.g.
//     sysVar(1) := 0;
//
// for/while/switch return a "not supported yet" error, as does any other
// construct; errors name the block and the offending text.
package zssexec

import (
	"fmt"
	"maps"
	"math"
	"strconv"
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/statemachine"
)

// Result is the outcome of one Step call.
type Result struct {
	// Context is the fighter's Context after running script's Statedef
	// body for ctx.StateNo.
	Context evaluator.Context
	// Sounds lists the sound events the script's PlaySnd controllers
	// recorded during this call, in order -- fresh per call, nil when there
	// were none, never left in Context (see statemachine.Result.Sounds).
	Sounds []evaluator.SoundEvent
	// Unsupported lists, lower-cased and deduplicated, the controller types
	// the script applied that the engine does not implement -- fresh per
	// call, nil when there were none (see statemachine.Result.Unsupported).
	Unsupported []string
	// SkippedCalls describes, in order, each `call` that did nothing because
	// the function is undefined or the argument or return count is wrong --
	// Ikemen GO logs it and runs on (bytecode.go:5050-5069). Fresh per call,
	// nil when there were none.
	SkippedCalls []string
	// HasHitDef reports whether a HitDef controller ran during this call;
	// HitDef is the first one, with every parameter that reads a local
	// ($name) already replaced by its value (see hitDefParams). The root
	// package acts on it as on a .cns HitDef.
	HasHitDef bool
	HitDef    cns.Controller
}

// CompiledScript is a character's full parsed .zss Script (zss.Script):
// every Statedef and Function body parsed once by Compile and indexed by
// state number and (lower-cased) function name (decision 013). Step takes a
// CompiledScript, never a raw zss.Script, so a caller holding one across a
// whole match pays the parse and the lookups' O(n) scan exactly once.
//
// A body that does not parse is kept with its error: Errors lists them all,
// and Step returns the error of the block it runs.
type CompiledScript struct {
	entries   []*compiledBlock
	statedefs map[int]*compiledBlock
	functions map[string]*compiledBlock
}

// Compile parses every block of script and indexes them. When script.Blocks
// holds more than one Statedef with the same number (or Function with the
// same name, compared case-insensitively as Ikemen GO does), the first one
// in declaration order is the one every lookup returns.
func Compile(script zss.Script) CompiledScript {
	prog := CompiledScript{
		statedefs: make(map[int]*compiledBlock),
		functions: make(map[string]*compiledBlock),
	}
	for _, b := range script.Blocks {
		cb := compileBlock(b)
		prog.entries = append(prog.entries, cb)
		switch b.Kind {
		case zss.BlockKindStatedef:
			// A symbolic or relative number (NumberRaw) has no state number
			// to index under; Number is 0 for it.
			if _, exists := prog.statedefs[b.Number]; !exists && b.NumberRaw == "" {
				prog.statedefs[b.Number] = cb
			}
		case zss.BlockKindFunction:
			name := strings.ToLower(b.Name)
			if _, exists := prog.functions[name]; !exists {
				prog.functions[name] = cb
			}
		}
	}
	return prog
}

// Header returns the Statedef header of state number as the cns.StateDef the
// root package's state entry takes (type, movetype, physics, anim, ctrl,
// ...), and whether prog defines that state. The state's controllers are not
// part of it: they live in the Statedef block's body, run by StepState.
func (prog CompiledScript) Header(number int) (cns.StateDef, bool) {
	block, ok := prog.statedefs[number]
	if !ok {
		return cns.StateDef{}, false
	}
	return block.header, true
}

// Errors returns the parse error of every block that has one, in file
// order, each naming its block.
func (prog CompiledScript) Errors() []error {
	var errs []error
	for _, cb := range prog.entries {
		if cb.err != nil {
			errs = append(errs, cb.err)
		}
	}
	return errs
}

// blocksParsed counts the blocks that parsed without error.
func (prog CompiledScript) blocksParsed() int {
	n := 0
	for _, cb := range prog.entries {
		if cb.err == nil {
			n++
		}
	}
	return n
}

// Step interprets one simulation tick of ctx's current state (ctx.StateNo)
// against prog, the character's full parsed .zss Script, precomputed once
// by Compile. It is StepState for the current state, with ChangeState
// targets checked against prog's own Statedefs.
func Step(ctx evaluator.Context, prog CompiledScript) (Result, error) {
	exists := func(n int) bool {
		_, ok := prog.statedefs[n]
		return ok
	}
	return StepState(ctx, ctx.StateNo, prog, exists)
}

// StepState runs the body of the Statedef block number from the top,
// evaluating each if/else condition and applying each controller-call
// statement in sequence, the same "stop at the first state change" rule
// statemachine.Step follows: once a ChangeState-shaped statement applies,
// no further statement in the body -- at any nesting level, including
// ones after the enclosing if block -- runs for this call. A state change
// also zeroes the persistent(N) counters (statemachine.ApplyController does
// it, as Ikemen GO resets them on every state entry, char.go:6896-6903).
// ctx itself is never mutated; StepState returns an independent, updated
// copy, whose Locals are empty again.
//
// number need not be ctx.StateNo: the special states -3, -2 and -1 run this
// way before the current state. exists reports whether a state number is
// defined in whatever the caller executes -- the root package passes its
// merged .cns and .zss table, as Ikemen GO changes state in one table
// (char.go changeState) -- and validates ChangeState targets only.
//
// StepState returns a descriptive error, never a panic, when number has no
// matching Statedef block in prog, when the body does not parse (see
// Errors), or when running it fails the same way statemachine.Step's own
// errors do (an unknown trigger name, a ChangeState target exists does not
// report, ...).
func StepState(ctx evaluator.Context, number int, prog CompiledScript, exists func(int) bool) (Result, error) {
	return stepState(ctx, number, prog, exists, false)
}

// StepStateDuringHitPause is StepState for a fighter frozen by a hit pause,
// as Ikemen GO's StateBlock.Run behaves (src/bytecode.go:5136-5260, commit
// 54d4148a): a block runs only if it is flagged ignoreHitPause (directly,
// by inheritance from an enclosing flagged block, or because it belongs to a
// Function); a skipped block does not even touch its persistent counter or
// evaluate its condition. The statements directly inside a block (controller
// calls, let, assignments and call) run only when the block's own
// ignoreHitPause makes them (ctrlsIgnorehitpause): a Statedef's top level
// never does, a Function's body always does.
func StepStateDuringHitPause(ctx evaluator.Context, number int, prog CompiledScript, exists func(int) bool) (Result, error) {
	return stepState(ctx, number, prog, exists, true)
}

func stepState(ctx evaluator.Context, number int, prog CompiledScript, exists func(int) bool, hitPause bool) (Result, error) {
	block, ok := prog.statedefs[number]
	if !ok {
		return Result{}, fmt.Errorf("zssexec: state %d not found in loaded .zss script", number)
	}
	if block.err != nil {
		return Result{}, fmt.Errorf("zssexec: %w", block.err)
	}

	working := ctx
	working.Sounds = nil
	working.Unsupported = working.Unsupported[:0]
	r := runner{ctx: &working, prog: prog, exists: exists, hitPause: hitPause}
	working.ZSS = true
	localTop = 0
	skippedCalls = skippedCalls[:0]
	hasHitDef, hitDef = false, cns.Controller{}
	root := frame{base: pushFrame(len(block.names)), names: block.names}
	working.Locals = root.locals()
	_, err := r.stmts(block.stmts, root, 0, !hitPause)
	if err != nil {
		return Result{}, fmt.Errorf("zssexec: state %d: %w", number, err)
	}
	working.Locals = evaluator.LocalFrame{}
	working.ZSS = false

	var skipped []string
	if len(skippedCalls) > 0 {
		skipped = append(skipped, skippedCalls...)
	}
	sounds := working.Sounds
	working.Sounds = nil
	unsupported := statemachine.TakeUnsupported(&working)
	return Result{Context: working, Sounds: sounds, Unsupported: unsupported, SkippedCalls: skipped, HasHitDef: hasHitDef, HitDef: hitDef}, nil
}

// maxCallDepth bounds how many nested "call F(...);" statements run will
// follow before giving up with a descriptive error. `.zss` script bodies
// are untrusted, mod-authored character content -- direct or mutual
// recursion would otherwise recurse the Go call stack without bound,
// crashing the process with an unrecoverable "fatal error: stack overflow"
// that no panic recovery (including cmd/wasm's own guarded) can intercept.
const maxCallDepth = 64

// localStack holds the local variables of the running Step: one frame per
// active block, from the Statedef's at the bottom to the innermost call's at
// localTop. It only grows (a call's frame is zeroed when pushed), so a warm
// Step allocates nothing. Unsynchronized like this package's other
// package-level state: engine targets a single-threaded GOOS=js GOARCH=wasm
// build with no goroutines in the simulation loop.
var (
	localStack []evaluator.Value
	localTop   int
)

// pushFrame reserves a zeroed frame of n values on localStack and returns
// its base. Growing replaces the backing array, so every live frame must be
// re-sliced from localStack by base afterwards (frame.locals does).
func pushFrame(n int) int {
	base := localTop
	need := base + n
	if need > len(localStack) {
		size := 2 * len(localStack)
		if size < need {
			size = need
		}
		grown := make([]evaluator.Value, size)
		copy(grown, localStack[:base])
		localStack = grown
	}
	clear(localStack[base:need])
	localTop = need
	return base
}

// frame is one block's window on localStack.
type frame struct {
	base  int
	names []string
}

func (f frame) locals() evaluator.LocalFrame {
	return evaluator.LocalFrame{Names: f.names, Values: localStack[f.base : f.base+len(f.names)]}
}

// runner is the state shared by every statement of one Step.
type runner struct {
	ctx      *evaluator.Context
	prog     CompiledScript
	exists   func(int) bool
	hitPause bool
}

// skippedCalls collects Result.SkippedCalls during a Step. It is a
// package-level variable, not a runner field: appending through the runner
// pointer makes the Context behind it escape to the heap on every Step.
var skippedCalls []string

// hitDef is the first HitDef that ran during a Step (hasHitDef), reported in
// Result. Package-level for skippedCalls' reason: read back through the
// runner, it would carry the Context to the heap with it.
var (
	hasHitDef bool
	hitDef    cns.Controller
)

// stmts runs stmts in order in frame fr (ctx.Locals is fr.locals() on entry
// and on exit), and reports whether a state change happened -- the caller
// must stop running any further statements at its own level (and every
// enclosing level) as soon as this is true. depth is the current call
// nesting level (see maxCallDepth); if/else blocks do not change it. Only
// blocks are run when ctrls is false (a hit pause the list's block does not
// ignore): Ikemen GO's StateBlock.Run skips every other statement then.
func (r *runner) stmts(stmts []stmt, fr frame, depth int, ctrls bool) (bool, error) {
	for _, s := range stmts {
		if _, isBlock := s.(*blockStmt); !isBlock && !ctrls {
			continue
		}
		stopped, err := r.stmt(s, fr, depth)
		if err != nil {
			return false, err
		}
		if stopped {
			return true, nil
		}
	}
	return false, nil
}

func (r *runner) stmt(s stmt, fr frame, depth int) (bool, error) {
	switch st := s.(type) {
	case *blockStmt:
		return r.block(st, fr, depth)

	case letStmt:
		for i, expr := range st.exprs {
			v, err := evaluator.EvaluateMutShortCircuit(expr, r.ctx)
			if err != nil {
				return false, fmt.Errorf("evaluating let expression %q: %w", expr, err)
			}
			if st.slots[i] >= 0 {
				localStack[fr.base+st.slots[i]] = v
			}
		}
		return false, nil

	case exprStmt:
		if _, err := evaluator.EvaluateMutShortCircuit(st.expr, r.ctx); err != nil {
			return false, fmt.Errorf("evaluating %q: %w", st.expr, err)
		}
		return false, nil

	case callStmt:
		return r.call(st, fr, depth)

	case ctrlStmt:
		if !hasHitDef && strings.EqualFold(st.ctrl.Type, statemachine.ControllerTypeHitDef) {
			params, err := hitDefParams(st.ctrl.Parameters, r.ctx)
			if err != nil {
				return false, fmt.Errorf("applying %s: %w", st.ctrl.Type, err)
			}
			hitDef = cns.Controller{Type: st.ctrl.Type, Triggers: st.ctrl.Triggers, Parameters: params}
			hasHitDef = true
		}
		changed, err := statemachine.ApplyController(st.ctrl, r.ctx, r.exists)
		if err != nil {
			return false, fmt.Errorf("applying %s: %w", st.ctrl.Type, err)
		}
		return changed, nil

	default:
		// Unreachable: the parser never produces any other stmt type.
		return false, fmt.Errorf("internal error: unknown statement type %T", s)
	}
}

// block runs a block as Ikemen GO's StateBlock.Run does (bytecode.go:5136):
// a persistent counter is decremented first (unless it holds the
// "once" value MaxInt32) and the block is skipped while it is above 0; the
// counter is set to N again only after a run that did not change state. In
// a hit pause a block that is not flagged ignoreHitPause does nothing at all
// (bytecode.go:5113-5120).
func (r *runner) block(b *blockStmt, fr frame, depth int) (bool, error) {
	if r.hitPause && !b.ignoreHitPause {
		return false, nil
	}
	if b.persistIdx >= 0 {
		if r.ctx.Persistent[b.persistIdx] != math.MaxInt32 {
			r.ctx.Persistent[b.persistIdx]--
		}
		if r.ctx.Persistent[b.persistIdx] > 0 {
			return false, nil
		}
	}
	if b.hasCond {
		v, err := evaluator.EvaluateMutShortCircuit(b.cond, r.ctx)
		if err != nil {
			return false, fmt.Errorf("evaluating if condition %q: %w", b.cond, err)
		}
		if !v.Bool() {
			if b.els != nil {
				return r.block(b.els, fr, depth)
			}
			return false, nil
		}
	}
	changed, err := r.stmts(b.body, fr, depth, !r.hitPause || b.ctrlsIgnoreHitPause)
	if err != nil || changed {
		return changed, err
	}
	if b.persistIdx >= 0 {
		r.ctx.Persistent[b.persistIdx] = b.persistN
	}
	return false, nil
}

// call runs a Function in a fresh zeroed frame (arguments, then return
// values, then lets) and, unless it changed state, copies its return values
// to the caller's slots (BytecodeFunction.run, bytecode.go:4998-5030), its
// body running whole even in a hit pause: the call itself was the check. A
// call to an undefined function, or with the wrong number of arguments or
// returns, is skipped and reported in Result.SkippedCalls, as Ikemen GO
// logs it and does nothing (bytecode.go:5050-5069).
func (r *runner) call(st callStmt, fr frame, depth int) (bool, error) {
	if depth >= maxCallDepth {
		return false, fmt.Errorf("call to %q exceeds maximum call depth %d (likely infinite recursion)", st.name, maxCallDepth)
	}
	fn, ok := r.prog.functions[st.name]
	if !ok {
		skippedCalls = append(skippedCalls, fmt.Sprintf("call to undefined function %q", st.name))
		return false, nil
	}
	if fn.err != nil {
		return false, fn.err
	}
	if len(st.args) != fn.nArgs {
		skippedCalls = append(skippedCalls, fmt.Sprintf("call to %q: %s expects %d arguments, got %d", st.name, fn.desc, fn.nArgs, len(st.args)))
		return false, nil
	}
	if len(st.rets) > 0 && len(st.rets) != fn.nRets {
		skippedCalls = append(skippedCalls, fmt.Sprintf("call to %q: %s returns %d values, %d expected", st.name, fn.desc, fn.nRets, len(st.rets)))
		return false, nil
	}

	callee := frame{base: pushFrame(len(fn.names)), names: fn.names}
	r.ctx.Locals = fr.locals() // pushFrame may have moved the stack
	for i, arg := range st.args {
		v, err := evaluator.EvaluateMutShortCircuit(arg, r.ctx)
		if err != nil {
			return false, fmt.Errorf("evaluating argument %q of call to %q: %w", arg, st.name, err)
		}
		localStack[callee.base+i] = v
	}
	r.ctx.Locals = callee.locals()
	changed, err := r.stmts(fn.stmts, callee, depth+1, true)
	localTop = callee.base
	r.ctx.Locals = fr.locals()
	if err != nil {
		return false, fmt.Errorf("%s: %w", fn.desc, err)
	}
	if changed {
		return true, nil
	}
	for i, slot := range st.rets {
		if slot >= 0 {
			localStack[fr.base+slot] = localStack[callee.base+fn.nArgs+i]
		}
	}
	return false, nil
}

// hitDefParams returns params with every comma-separated value that reads a
// local ($name) replaced by its value, evaluated now with ctx's locals:
// Ikemen GO evaluates a HitDef's parameters when the controller runs
// (hitDef.Run, src/bytecode.go:8631-8650, runSub :8242), and a local exists
// only while its block runs. Any other value stays raw text, read later by
// combat.ParseHitDef as for a .cns HitDef (decision 030). params itself is
// returned, nothing allocated, when no value reads a local.
func hitDefParams(params map[string]string, ctx *evaluator.Context) (map[string]string, error) {
	resolved := params
	cloned := false
	for key, raw := range params {
		if !strings.Contains(raw, "$") {
			continue
		}
		parts := strings.Split(raw, ",")
		for i, part := range parts {
			if !strings.Contains(part, "$") {
				continue
			}
			v, err := evaluator.EvaluateMutShortCircuit(part, ctx)
			if err != nil {
				return nil, fmt.Errorf("parameter %q value %q: %w", key, part, err)
			}
			parts[i] = strconv.FormatFloat(v.Number, 'f', -1, 64)
		}
		if !cloned {
			resolved = maps.Clone(params)
			cloned = true
		}
		resolved[key] = strings.Join(parts, ",")
	}
	return resolved, nil
}
