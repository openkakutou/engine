// Package statemachine drives a fighter's state transitions by
// interpreting character/cns's StateDef/Controller data using the trigger
// evaluator (engine/evaluator) against a fighter's live evaluator.Context.
//
// Each call to Step evaluates one simulation tick of the fighter's current
// state: its controllers are checked, in declared order, against the given
// Context, and every controller whose trigger conditions evaluate true is
// applied, updating the Context as it goes so that a later controller's
// trigger can observe an earlier one's effect within the same call. This
// package supports a growing set of controller types: ChangeState (state
// transition), VarSet (variable assignment), PowerAdd (power/meter gain or
// spend, clamped to [0, DefaultMaxPower]), PlaySnd (recording a triggered
// (group, sample) sound event -- engine never decodes or plays the audio
// itself), and the movement and state controllers of movement.go (VelSet,
// VelAdd, VelMul, PosSet, PosAdd, ChangeAnim, CtrlSet, StateTypeSet, Turn,
// LifeAdd, LifeSet, Gravity, Null). A controller of an unimplemented type
// is still recorded as "applied" when its trigger evaluates true, has no
// effect on the Context, and is reported by name in Result.Unsupported.
//
// A controller fires when every TriggerAll line holds and at least one of
// its TriggerGroups ("trigger1", "trigger2", ...) holds, a group holding when
// all its lines do -- evaluated in Ikemen GO's order with its short-circuit.
// See .vibe/decisions/032-controller-trigger-groups-ored-as-ikemen-go.md.
//
// Matching real MUGEN/Ikemen behavior, once a ChangeState controller
// applies, Step stops evaluating any further controllers in the current
// state for that call -- control has already moved to the new state.
package statemachine

import (
	"fmt"
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

// Controller type names this package recognizes, declared once here rather
// than as separate string literals in every file that needs to classify a
// cns.Controller (ApplyController's own switch below, and the root engine
// package's tick.go, which needs to recognize a HitDef controller to drive
// combat) -- a typo in one of two independently hardcoded literals would
// otherwise fail silently at runtime instead of at compile time.
const (
	ControllerTypeChangeState = "ChangeState"
	ControllerTypeVarSet      = "VarSet"
	ControllerTypeHitDef      = "HitDef"
	ControllerTypePowerAdd    = "PowerAdd"
	ControllerTypePlaySnd     = "PlaySnd"

	ControllerTypeVelSet       = "VelSet"
	ControllerTypeVelAdd       = "VelAdd"
	ControllerTypeVelMul       = "VelMul"
	ControllerTypePosSet       = "PosSet"
	ControllerTypePosAdd       = "PosAdd"
	ControllerTypeChangeAnim   = "ChangeAnim"
	ControllerTypeCtrlSet      = "CtrlSet"
	ControllerTypeStateTypeSet = "StateTypeSet"
	ControllerTypeTurn         = "Turn"
	ControllerTypeLifeAdd      = "LifeAdd"
	ControllerTypeLifeSet      = "LifeSet"
	ControllerTypeGravity      = "Gravity"
	ControllerTypeNull         = "Null"
	ControllerTypePlayerPush   = "PlayerPush"
)

// DefaultMaxPower is the power/meter cap applied when clamping a
// PowerAdd controller's effect -- MUGEN's own default player power cap.
// engine currently has no lifebar/system-config data flowing into it to
// override this with (its only cross-repo inputs are character and
// stage), so this stays a hardcoded constant rather than a configurable
// parameter until a real such data source exists. See
// .vibe/decisions/015.
const DefaultMaxPower = 3000

// Result is the outcome of one Step call.
type Result struct {
	// Context is the fighter's Context after applying every controller
	// that triggered true during this call.
	Context evaluator.Context
	// Applied lists the indices, into the current state's Controllers
	// slice, of every controller whose trigger conditions evaluated true
	// this call, in declared order. An index appears here even if the
	// controller's type has no implemented effect.
	Applied []int
	// Sounds lists every SoundEvent a PlaySnd controller recorded during
	// this call, in trigger order -- fresh, this-call-only data exactly
	// like Applied, never carried over from Context between calls (see
	// evaluator.Context.Sounds's own doc comment and
	// .vibe/decisions/017).
	Sounds []evaluator.SoundEvent
	// Unsupported lists, lower-cased and deduplicated in first-seen order,
	// the types of the controllers that applied this call without an
	// implementation (HitDef excepted: the root package's Tick acts on it).
	// Fresh per call like Applied and Sounds; nil when there were none. It
	// aliases a buffer the Context's own Unsupported keeps between calls, so
	// it is only valid until the next call that starts from Context. See
	// .vibe/decisions/020.
	Unsupported []string
	// Warnings lists the Ikemen GO console warnings of this call, in order
	// ("changed to invalid state N (from state M)", char.go:6880-6887); nil
	// when there were none.
	Warnings []string
}

// Step interprets one simulation tick of ctx's current state (ctx.StateNo)
// against states, the complete set of StateDefs for the fighter's loaded
// character, keyed by StateDef.Number.
//
// It evaluates that state's controllers in declared order, applying the
// effect of every controller whose trigger conditions evaluate true and
// skipping those that evaluate false, and returns the resulting Context.
// Processing stops early -- leaving any remaining controllers unevaluated
// -- as soon as a ChangeState controller applies. ctx itself is never
// mutated; Step returns an independent, updated copy.
//
// Step returns a descriptive error, never a panic, when ctx.StateNo is not
// present in states, when a controller's trigger expression fails to
// evaluate (e.g. an unknown trigger name), or when a ChangeState
// controller's target state is missing its "value" parameter, that
// parameter fails to evaluate, or the resulting target state number is not
// present in states.
func Step(ctx evaluator.Context, states map[int]cns.StateDef) (Result, error) {
	if _, ok := states[ctx.StateNo]; !ok {
		return Result{}, fmt.Errorf("statemachine: current state %d not found in loaded character", ctx.StateNo)
	}
	return StepState(ctx, ctx.StateNo, states)
}

// StepState runs the controllers of state number against ctx exactly as
// Step does for the current state, but for any state of the character --
// the special states -3, -2 and -1 are run this way, ahead of the current
// state, without becoming ctx's current state. It returns an error when
// number is not present in states.
func StepState(ctx evaluator.Context, number int, states map[int]cns.StateDef) (Result, error) {
	return stepState(ctx, number, states, func(n int) bool { _, ok := states[n]; return ok }, false)
}

// StepStateWith is StepState for a character whose states are not all in
// states: exists reports whether a state number is defined anywhere (here
// and in the .zss script of the same character), and validates ChangeState
// targets only -- Ikemen GO changes state in one table of .cns and .zss
// states (char.go changeState). number itself must be in states.
func StepStateWith(ctx evaluator.Context, number int, states map[int]cns.StateDef, exists func(int) bool) (Result, error) {
	return stepState(ctx, number, states, exists, false)
}

// StepStateDuringHitPause is StepState for a fighter frozen by a hit pause:
// only the controllers whose "ignorehitpause" parameter evaluates true are
// considered (their triggers are evaluated as usual); all others are
// skipped without evaluating their triggers. MUGEN 1.1 cns doc: a
// controller without ignorehitpause "will not be checked during a hit
// pause"; Ikemen GO wraps such blocks in ignoreHitPause{}.
func StepStateDuringHitPause(ctx evaluator.Context, number int, states map[int]cns.StateDef) (Result, error) {
	return stepState(ctx, number, states, func(n int) bool { _, ok := states[n]; return ok }, true)
}

// StepStateDuringHitPauseWith is StepStateDuringHitPause with the existence
// check of StepStateWith.
func StepStateDuringHitPauseWith(ctx evaluator.Context, number int, states map[int]cns.StateDef, exists func(int) bool) (Result, error) {
	return stepState(ctx, number, states, exists, true)
}

func stepState(ctx evaluator.Context, number int, states map[int]cns.StateDef, exists func(int) bool, hitPause bool) (Result, error) {
	def, ok := states[number]
	if !ok {
		return Result{}, fmt.Errorf("statemachine: state %d not found in loaded character", number)
	}

	working := ctx
	// Sounds is this call's own event log, never Context's carried-forward
	// state -- reset here so a previous tick's already-reported events
	// (still sitting in ctx.Sounds because the caller threads Context
	// tick-to-tick via FighterRuntime) can never be mistaken for this
	// call's. See evaluator.Context.Sounds's own doc comment.
	working.Sounds = nil
	working.Warnings = nil
	working.Unsupported = working.Unsupported[:0]
	var applied []int

	for i, ctrl := range def.Controllers {
		if hitPause {
			run, err := ignoresHitPause(ctrl, &working)
			if err != nil {
				return Result{}, fmt.Errorf("statemachine: state %d controller %d (%s): evaluating ignorehitpause: %w", number, i, ctrl.Type, err)
			}
			if !run {
				continue
			}
		}
		pass, err := triggersPass(ctrl, &working)
		if err != nil {
			return Result{}, fmt.Errorf("statemachine: state %d controller %d (%s): evaluating triggers: %w", number, i, ctrl.Type, err)
		}
		if !pass {
			continue
		}
		applied = append(applied, i)

		changedState, err := ApplyController(ctrl, &working, exists)
		if err != nil {
			return Result{}, fmt.Errorf("statemachine: state %d controller %d (%s): %w", number, i, ctrl.Type, err)
		}
		if changedState {
			break
		}
	}

	sounds := working.Sounds
	working.Sounds = nil
	warnings := working.Warnings
	working.Warnings = nil
	unsupported := TakeUnsupported(&working)
	return Result{Context: working, Applied: applied, Sounds: sounds, Unsupported: unsupported, Warnings: warnings}, nil
}

// ignoresHitPause evaluates "ignorehitpause" as an expression, not a
// literal 0/1.
func ignoresHitPause(ctrl cns.Controller, ctx *evaluator.Context) (bool, error) {
	raw, ok := ctrl.Parameters["ignorehitpause"]
	if !ok {
		return false, nil
	}
	v, err := evaluator.EvaluateMut(raw, ctx)
	if err != nil {
		return false, err
	}
	return v.Bool(), nil
}

// triggersPass reports whether ctrl's trigger condition holds, in Ikemen
// GO's evaluation order (stateCompileCNS, src/compiler.go:7442-7495, commit
// 54d4148a): every TriggerAll line first, stopping at the first false one,
// then the groups in order, each stopping at its first false line, the
// whole stopping at the first true group -- so a later line's ":=" or
// Random never runs once the outcome is known. Non-empty Triggers with no
// TriggerGroups means no trigger1, which Ikemen GO refuses ("Missing
// trigger1", src/compiler.go:7439-7441): such a controller never fires.
//
// shortcut: a controller with neither TriggerAll nor TriggerGroups (built
// by hand, not by cns.Parse) falls back to ANDing its flat Triggers, and
// an empty Triggers is unconditional, as before groups existed.
func triggersPass(ctrl cns.Controller, ctx *evaluator.Context) (bool, error) {
	if len(ctrl.TriggerAll) == 0 && len(ctrl.TriggerGroups) == 0 {
		return allHold(ctrl.Triggers, ctx)
	}
	pass, err := allHold(ctrl.TriggerAll, ctx)
	if err != nil || !pass {
		return false, err
	}
	for _, group := range ctrl.TriggerGroups {
		pass, err := allHold(group, ctx)
		if err != nil {
			return false, err
		}
		if pass {
			return true, nil
		}
	}
	return false, nil
}

// allHold reports whether every expression in lines evaluates true,
// stopping at the first false one; an empty list holds.
func allHold(lines []string, ctx *evaluator.Context) (bool, error) {
	for _, line := range lines {
		v, err := evaluator.EvaluateMut(line, ctx)
		if err != nil {
			return false, err
		}
		if !v.Bool() {
			return false, nil
		}
	}
	return true, nil
}

// ControllerHandler computes one non-state-changing controller type's
// effect, taking ctx by value and returning the updated copy -- never a
// pointer -- so ApplyController can dispatch to it through this package's
// registry without defeating the Go compiler's escape analysis for
// Step's per-call working copy: a pointer parameter flowing into an
// indirect call is conservatively treated as leaking, which forced
// Step's Context to the heap on every tick when this was tried with a
// pointer-based handler shape -- measured, not assumed; see
// .vibe/decisions/016. ChangeState is deliberately not shaped as a
// ControllerHandler and is not registered through RegisterController:
// unlike every other controller type, it also needs the exists predicate
// to validate its target and it terminates Step's controller loop early,
// so it stays ApplyController's one direct, statically-dispatched special
// case.
type ControllerHandler func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error)

// controllerHandlers maps a lower-cased controller type name (excluding
// ChangeState, see ControllerHandler's doc comment) to the handler that
// applies its effect. Registered once below via RegisterController for the
// two types this package implements today -- growing this set is an
// addition to this registration, never a change to ApplyController's own
// code (unlike a switch statement, which would need a new case per type).
// Unsynchronized like this package's other package-level state: engine
// targets a single-threaded GOOS=js GOARCH=wasm build with no goroutines
// in the simulation loop.
var controllerHandlers = map[string]ControllerHandler{}

func init() {
	RegisterController(ControllerTypeVarSet, applyVarSet)
	RegisterController("VarAdd", applyVarAdd)
	RegisterController("VarRangeSet", applyVarRangeSet)
	RegisterController(ControllerTypePowerAdd, applyPowerAdd)
	RegisterController(ControllerTypePlaySnd, applyPlaySnd)
	RegisterController("MapSet", func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
		return applyMapSet(ctrl, ctx, false)
	})
	RegisterController("MapAdd", func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
		return applyMapSet(ctrl, ctx, true)
	})
}

// applyMapSet is MapSet (add false) or MapAdd in their "map: name; value:
// expr" form (compiler_functions.go:4768-4900, char.go:9808-9852): the name,
// quoted or bare, is lower-cased; MapAdd adds to the current value, 0 when
// unset.
// shortcut: the map(name) := value and indexed forms are not read, and the
// Parent, Root and Team variants report unsupported; upgrade when a
// character needs them.
func applyMapSet(ctrl cns.Controller, ctx evaluator.Context, add bool) (evaluator.Context, error) {
	name := strings.Trim(strings.TrimSpace(ctrl.Parameters["map"]), `"`)
	if name == "" {
		return ctx, fmt.Errorf("%s needs a map name", ctrl.Type)
	}
	raw, ok := ctrl.Parameters["value"]
	if !ok {
		return ctx, fmt.Errorf("%s needs a value", ctrl.Type)
	}
	v, err := evaluator.EvaluateMut(raw, &ctx)
	if err != nil {
		return ctx, fmt.Errorf("%s value %q: %w", ctrl.Type, raw, err)
	}
	if add {
		v, err = evaluator.Add(ctx.MapValue(name), v)
		if err != nil {
			return ctx, err
		}
	}
	ctx.SetMap(name, v)
	return ctx, nil
}

// RegisterController associates a controller type name (matched
// case-insensitively, matching real .cns files' inconsistent casing) with
// the handler that applies its effect. Called during package
// initialization for this package's own built-in types; also usable by
// any future non-state-changing controller type -- built into this
// package or, if ever needed, added from outside it -- without modifying
// ApplyController's code. Re-registering the same type name
// (case-insensitively) replaces its previous handler. Does not accept
// ChangeState; see ControllerHandler's doc comment.
func RegisterController(controllerType string, handler ControllerHandler) {
	controllerHandlers[strings.ToLower(controllerType)] = handler
}

// IsSupportedController reports whether controllerType (case-insensitive)
// is ChangeState or a type registered through RegisterController.
func IsSupportedController(controllerType string) bool {
	t := strings.ToLower(controllerType)
	if t == strings.ToLower(ControllerTypeChangeState) {
		return true
	}
	_, ok := controllerHandlers[t]
	return ok
}

// ApplyController applies ctrl's effect to ctx, mutating it in place. It
// reports whether the effect was a state change (ChangeState), so the
// caller can stop processing further controllers for this call. exists
// reports whether a given state number is defined in whatever the caller is
// currently executing -- used only to validate a ChangeState target.
//
// ChangeState is handled directly (see ControllerHandler's doc comment);
// every other type dispatches through whatever handler was registered for
// ctrl.Type via RegisterController.
//
// Exported (rather than kept as Step's own private helper) so `.zss`
// execution (engine/zssexec) can apply this same small set of controller
// types to a controller-shaped statement without duplicating this logic --
// see that package's own doc comment and .vibe/decisions/008 in this repo.
//
// A controller type with no registered handler has no effect on ctx but is
// not silent: its lower-cased name is added to ctx.Unsupported (once per
// type), which Step surfaces as Result.Unsupported. HitDef is the exception:
// it has no handler here because the root package's Tick acts on it.
func ApplyController(ctrl cns.Controller, ctx *evaluator.Context, exists func(int) bool) (bool, error) {
	if strings.EqualFold(ctrl.Type, ControllerTypeChangeState) {
		return true, applyChangeState(ctrl, ctx, exists)
	}
	t := lowerType(ctrl.Type)
	handler, ok := controllerHandlers[t]
	if !ok {
		reportUnsupported(ctx, t)
		return false, nil
	}
	updated, err := handler(ctrl, *ctx)
	if err != nil {
		return false, err
	}
	*ctx = updated
	return false, nil
}

func applyChangeState(ctrl cns.Controller, ctx *evaluator.Context, exists func(int) bool) error {
	raw, ok := ctrl.Parameters["value"]
	if !ok {
		return fmt.Errorf(`ChangeState is missing its required "value" parameter`)
	}
	v, err := evaluator.EvaluateMut(raw, ctx)
	if err != nil {
		return fmt.Errorf("ChangeState value %q: %w", raw, err)
	}
	target := v.Int()
	// A target the character does not have is not an error: Ikemen GO warns
	// and enters an empty state (changeStateEx, char.go:6880-6887); the
	// caller enters it with unchanged types and physics.
	if !exists(target) {
		ctx.Warnings = append(ctx.Warnings, fmt.Sprintf("changed to invalid state %d (from state %d)", target, ctx.StateNo))
	}
	// Ikemen GO changeStateEx (char.go:6976-6983): an "anim" other than -1
	// restarts that animation and a non-negative "ctrl" sets the control,
	// both before the new state's header runs.
	anim, hasAnim, err := evalOptional(ctrl, "anim", ctx)
	if err != nil {
		return err
	}
	if hasAnim && int(anim) != -1 {
		ctx.Anim = int(anim)
		if ctx.Anim < 0 && ctx.Anim != invisibleAnim {
			ctx.Anim = 0
		}
		ctx.AnimElapsed = 0
		ctx.AnimElem, ctx.AnimElemTime = 0, 0
	}
	ctrlValue, hasCtrl, err := evalOptional(ctrl, "ctrl", ctx)
	if err != nil {
		return err
	}
	if hasCtrl && ctrlValue >= 0 {
		ctx.Ctrl = ctrlValue != 0
	}

	ctx.PrevStateNo = ctx.StateNo
	ctx.StateNo = target
	ctx.Time = 0
	ctx.StateChanged = true
	// Ikemen GO zeroes the persistent(N) counters on every state entry
	// (char.go:6896-6903).
	ctx.Persistent = [evaluator.MaxPersistent]int32{}
	return nil
}

// varSetForm is one "var(N) = expr"-style VarSet parameter.
type varSetForm struct {
	prefix   string // "var", "sysvar", "fvar" or "sysfvar"
	indexRaw string
	valueRaw string
}

// varSetPrefixes is the order Ikemen GO matches a key's prefix in
// (src/compiler_functions.go:2739-2876 varSetNewerSub/varSetSub): the longer
// "sys..." names first so "sysvar(" is not read as "var(".
var varSetPrefixes = []string{"sysfvar", "sysvar", "fvar", "var"}

// parseVarSetKey reports whether key is a "var(N)"-style parameter name and
// returns its prefix and raw index expression. Matching is case-insensitive,
// spaces may precede "(", the index runs to the matching ")" and anything
// after it is ignored, as Ikemen GO does (compiler_functions.go:2814-2876).
func parseVarSetKey(key string) (prefix, index string, ok bool) {
	lower := strings.ToLower(strings.TrimSpace(key))
	for _, p := range varSetPrefixes {
		rest, found := strings.CutPrefix(lower, p)
		if !found {
			continue
		}
		rest = strings.TrimLeft(rest, " \t")
		if !strings.HasPrefix(rest, "(") {
			continue
		}
		depth := 0
		for i, r := range rest {
			if r == '(' {
				depth++
			}
			if r == ')' {
				depth--
			}
			if depth == 0 {
				return p, rest[1:i], true
			}
		}
		return p, "", false
	}
	return "", "", false
}

// applyVarSet sets one variable, in either form Ikemen GO accepts: a single
// "var(N) = expr" style parameter, or "v"/"fv"/"sysv"/"sysfv" = N plus
// "value" = expr (varSetOlderSub, compiler_functions.go:2705-2735). Exactly
// one variable may be named, else "VarSet can only set one variable at a
// time". A negative or too-large index is an error (Ikemen GO only warns on
// a negative one, char.go:8295-8365). It runs on the per-tick hot path, so
// it tracks the first match and a count instead of building a slice.
func applyVarSet(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	return applyVarAssign(ctrl, ctx, "VarSet", false)
}

// applyVarAdd is VarAdd: VarSet's forms, the value added to the variable
// (compiler_functions.go:2889-2898, varSetSub with add). An int variable
// adds the value truncated to an int first, as Ikemen GO compiles it.
func applyVarAdd(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	return applyVarAssign(ctrl, ctx, "VarAdd", true)
}

func applyVarAssign(ctrl cns.Controller, ctx evaluator.Context, name string, add bool) (evaluator.Context, error) {
	var form varSetForm
	count := 0
	for key, raw := range ctrl.Parameters {
		prefix, index, ok := parseVarSetKey(key)
		if ok {
			form = varSetForm{prefix: prefix, indexRaw: index, valueRaw: raw}
			count++
		}
	}
	oldForm := false
	for _, old := range [...]struct{ key, prefix string }{{"v", "var"}, {"fv", "fvar"}, {"sysv", "sysvar"}, {"sysfv", "sysfvar"}} {
		raw, ok := ctrl.Parameters[old.key]
		if ok {
			form = varSetForm{prefix: old.prefix, indexRaw: raw}
			oldForm = true
			count++
		}
	}
	if count > 1 {
		return evaluator.Context{}, fmt.Errorf("%s can only set one variable at a time", name)
	}
	if count == 0 {
		return evaluator.Context{}, fmt.Errorf("%s needs a variable: var(N), sysvar(N), fvar(N), sysfvar(N), or v/sysv/fv/sysfv and value", name)
	}
	if oldForm {
		raw, ok := ctrl.Parameters["value"]
		if !ok {
			return evaluator.Context{}, fmt.Errorf(`%s is missing its required "value" parameter`, name)
		}
		form.valueRaw = raw
	}

	if strings.TrimSpace(form.indexRaw) == "" {
		return evaluator.Context{}, fmt.Errorf("%s: Missing variable index", name)
	}
	idxVal, err := evaluator.EvaluateMut(form.indexRaw, &ctx)
	if err != nil {
		return evaluator.Context{}, fmt.Errorf("%s %s index %q: %w", name, form.prefix, form.indexRaw, err)
	}
	val, err := evaluator.EvaluateMut(form.valueRaw, &ctx)
	if err != nil {
		return evaluator.Context{}, fmt.Errorf("%s %s value %q: %w", name, form.prefix, form.valueRaw, err)
	}
	if add {
		cur, err := ctx.VarValue(form.prefix, idxVal.Int())
		if err != nil {
			return evaluator.Context{}, fmt.Errorf("%s: %w", name, err)
		}
		if cur.Kind == evaluator.KindInt {
			val = evaluator.IntValue(val.Int())
		}
		if val, err = evaluator.Add(cur, val); err != nil {
			return evaluator.Context{}, fmt.Errorf("%s: %w", name, err)
		}
	}
	if _, err := ctx.AssignVar(form.prefix, idxVal.Int(), val); err != nil {
		return evaluator.Context{}, fmt.Errorf("%s: %w", name, err)
	}
	return ctx, nil
}

// applyVarRangeSet is VarRangeSet (bytecode.go:12025-12050, char.go:
// 8424-8476): var(first..last) to value, or fvar(first..last) to fvalue;
// first defaults to 0 and last to 59, and a range with first < 0 or first >
// last sets nothing. Indices past the variables the fighter has are left
// out (Ikemen GO's variables are unbounded maps).
func applyVarRangeSet(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	first, _, err := evalOptional(ctrl, "first", &ctx)
	if err != nil {
		return ctx, err
	}
	last, hasLast, err := evalOptional(ctrl, "last", &ctx)
	if err != nil {
		return ctx, err
	}
	if !hasLast {
		last = 59 // Ikemen GO's legacy default
	}
	value, isInt, err := evalOptional(ctrl, "value", &ctx)
	if err != nil {
		return ctx, err
	}
	target := "var"
	if !isInt {
		var isFloat bool
		value, isFloat, err = evalOptional(ctrl, "fvalue", &ctx)
		if err != nil {
			return ctx, err
		}
		if !isFloat {
			return ctx, fmt.Errorf("VarRangeSet value parameter not specified")
		}
		target = "fvar"
	}
	lo, hi := int(first), int(last)
	if lo < 0 || lo > hi {
		return ctx, nil
	}
	size := len(ctx.Vars)
	v := evaluator.IntValue(int(value))
	if target == "fvar" {
		size = len(ctx.FVars)
		v = evaluator.FloatValue(value)
	}
	for i := lo; i <= min(hi, size-1); i++ {
		if _, err := ctx.AssignVar(target, i, v); err != nil {
			return ctx, fmt.Errorf("VarRangeSet: %w", err)
		}
	}
	return ctx, nil
}

// applyPowerAdd applies a PowerAdd controller's effect to ctx: its "value"
// parameter (an evaluated MUGEN trigger expression, positive or negative)
// is added to ctx.Power, then the result is clamped to [0, DefaultMaxPower]
// -- never negative, never above the cap -- matching real MUGEN/Ikemen
// behavior for both gaining power on a landed hit and spending it on a
// super move.
func applyPowerAdd(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	raw, ok := ctrl.Parameters["value"]
	if !ok {
		return evaluator.Context{}, fmt.Errorf(`PowerAdd is missing its required "value" parameter`)
	}
	v, err := evaluator.EvaluateMut(raw, &ctx)
	if err != nil {
		return evaluator.Context{}, fmt.Errorf("PowerAdd value %q: %w", raw, err)
	}

	power := ctx.Power + v.Int()
	if power < 0 {
		power = 0
	}
	if power > DefaultMaxPower {
		power = DefaultMaxPower
	}
	ctx.Power = power
	return ctx, nil
}

// playSndValue is a PlaySnd "value" parameter split once: its sound-file
// prefix (0, 'f', 's' or 'm', lower-cased) and its two "group,sample"
// half-expressions with the prefix removed.
type playSndValue struct {
	prefix byte
	halves [2]string
}

// playSndValueCache memoizes splitting a PlaySnd controller's raw "value"
// parameter into its prefix and two half-expressions, keyed by the raw
// string itself -- the same per-tick reparse-avoidance convention
// evaluator.Evaluate's own parseCache and zssexec's bodyCache already
// follow (see their doc comments): a controller's raw parameter text is
// loaded once per character and never changes for the life of a match, so
// re-splitting it on every tick a PlaySnd fires would reintroduce exactly
// the per-tick string/slice allocation those caches exist to avoid. A
// malformed value (missing comma) is deliberately not cached -- see
// splitPlaySndValue. Safe as a plain, unsynchronized map for the same
// single-threaded, no-goroutines reason those caches are.
var playSndValueCache = make(map[string]playSndValue)

// SplitPlaySndValue is splitPlaySndValue's two half-expressions for callers
// outside the package (the conformance harness inspects each half).
func SplitPlaySndValue(raw string) ([2]string, error) {
	value, err := splitPlaySndValue(raw)
	return value.halves, err
}

// splitPlaySndValue removes raw's sound-file prefix (see playSndPrefix) and
// splits the rest into its two "group,sample" half-expressions (trimmed of
// surrounding whitespace), reusing playSndValueCache on repeat calls with
// the same raw string. Returns a descriptive error, never cached, when raw
// has no comma to split on.
func splitPlaySndValue(raw string) (playSndValue, error) {
	if value, ok := playSndValueCache[raw]; ok {
		return value, nil
	}
	prefix, rest := playSndPrefix(strings.TrimSpace(raw))
	idx := strings.IndexByte(rest, ',')
	if idx < 0 {
		return playSndValue{}, fmt.Errorf(`PlaySnd value %q must be "group,sample"`, raw)
	}
	value := playSndValue{prefix: prefix, halves: [2]string{strings.TrimSpace(rest[:idx]), strings.TrimSpace(rest[idx+1:])}}
	playSndValueCache[raw] = value
	return value, nil
}

// playSndPrefix is Ikemen GO's CharCompiler.getDataPrefix (src/compiler.go:
// 6425-6476, commit 54d4148a, called with ffxDefault false by playSnd,
// src/compiler_functions.go:363): a value longer than one character that
// starts with F, S or M (any case) loses that letter when the letters right
// after it are none ("S5", "S $bank") or a known trigger ("Fvar(0)"), so
// "StateNo" stays an expression. Returns the lower-cased prefix, or 0, and
// the rest of value.
//
// shortcut: Ikemen GO also matches the prefixes of the extra fightfx files
// a fight.def or a character declares (sys.ffx); the engine loads none, so
// only F, S and M are recognized.
func playSndPrefix(value string) (byte, string) {
	if len(value) < 2 {
		return 0, value
	}
	prefix := value[0] | 0x20 // ASCII lower case
	if prefix != 'f' && prefix != 's' && prefix != 'm' {
		return 0, value
	}
	rest := value[1:]
	end := 0
	for end < len(rest) && isASCIILetter(rest[end]) {
		end++
	}
	if end > 0 {
		token := strings.ToLower(rest[:end])
		if !evaluator.IsKnownTrigger(token, false) && !evaluator.IsKnownTrigger(token, true) {
			return 0, value
		}
	}
	return prefix, rest
}

func isASCIILetter(b byte) bool {
	lower := b | 0x20
	return lower >= 'a' && lower <= 'z'
}

// applyPlaySnd applies a PlaySnd controller's effect: its "value" parameter
// ("group,sample", each half a MUGEN trigger expression, not necessarily a
// literal, optionally prefixed by F, S or M -- see playSndPrefix) is split
// and evaluated against ctx, and the resolved evaluator.SoundEvent is
// appended to ctx.Sounds -- engine only records which sample was triggered;
// it never decodes or plays the audio itself (see .vibe/decisions/017).
//
// Of the parameters Ikemen GO's playSnd.Run reads (src/bytecode.go:
// 5671-5717), the event carries channel and lowpriority, which decide
// whether the sound stops the one playing on its channel. Ignored for now:
// redirectid, pan, abspan, volume, volumescale, freqmul, loop, priority,
// loopstart, loopend, startposition, loopcount, stopongethit,
// stoponchangestate.
func applyPlaySnd(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	raw, ok := ctrl.Parameters["value"]
	if !ok {
		return evaluator.Context{}, fmt.Errorf(`PlaySnd is missing its required "value" parameter`)
	}
	value, err := splitPlaySndValue(raw)
	if err != nil {
		return evaluator.Context{}, err
	}
	parts := value.halves

	groupVal, err := evaluator.EvaluateMut(parts[0], &ctx)
	if err != nil {
		return evaluator.Context{}, fmt.Errorf("PlaySnd value group %q: %w", parts[0], err)
	}
	sampleVal, err := evaluator.EvaluateMut(parts[1], &ctx)
	if err != nil {
		return evaluator.Context{}, fmt.Errorf("PlaySnd value sample %q: %w", parts[1], err)
	}
	event := evaluator.SoundEvent{Group: groupVal.Int(), Sample: sampleVal.Int(), Common: value.prefix == 'f', Channel: -1}
	if expr, ok := ctrl.Parameters["channel"]; ok {
		channel, err := evaluator.EvaluateMut(expr, &ctx)
		if err != nil {
			return evaluator.Context{}, fmt.Errorf("PlaySnd channel %q: %w", expr, err)
		}
		event.Channel = channel.Int()
	}
	if expr, ok := ctrl.Parameters["lowpriority"]; ok {
		lowPriority, err := evaluator.EvaluateMut(expr, &ctx)
		if err != nil {
			return evaluator.Context{}, fmt.Errorf("PlaySnd lowpriority %q: %w", expr, err)
		}
		event.LowPriority = lowPriority.Bool()
	}
	// The M prefix reads sys.ffx["m"] (Ikemen GO src/char.go:6660), which
	// nothing sets at commit 54d4148a: no sound plays, so none is recorded.
	if value.prefix == 'm' {
		return ctx, nil
	}

	ctx.Sounds = append(ctx.Sounds, event)
	return ctx, nil
}
