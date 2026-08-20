// Package zssexec executes character/zss's parsed Statedef/Function
// script bodies against a fighter's live evaluator.Context -- the .zss
// counterpart to engine/statemachine's Step for .cns-driven state
// execution.
//
// .zss's script body grammar is not standard Lua despite being commonly
// described as "Lua-like" -- see .vibe/decisions/008 in this repo for the
// real observed grammar and why this package is a small hand-written
// interpreter rather than an embedded Lua VM. Supported statement forms:
//
//   - if COND { ... } [else { ... }]        -- COND reuses engine/evaluator's
//     MUGEN trigger-expression grammar unchanged (the observed .zss
//     condition syntax already uses the same "=" comparison / "&&"/"||"
//     operators)
//   - Name{key: value; key2: value2; ...}   -- a single-line controller-call
//     statement, applied via statemachine.ApplyController (the same
//     ChangeState/VarSet logic .cns-driven states use)
//   - call FunctionName();                  -- calls a zero-parameter,
//     no-return-value Function block by name, executing its body the same
//     way
//
// A construct this package does not yet support -- a "let" assignment, a
// call to a function declaring parameters or a return value, or any line
// matching neither statement form above -- returns a descriptive error
// rather than silently doing nothing or panicking.
package zssexec

import (
	"fmt"

	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/statemachine"
)

// Result is the outcome of one Step call.
type Result struct {
	// Context is the fighter's Context after running script's Statedef
	// body for ctx.StateNo.
	Context evaluator.Context
}

// Step interprets one simulation tick of ctx's current state (ctx.StateNo)
// against script, the character's full parsed .zss Script (its Statedef
// and Function blocks).
//
// It runs that Statedef block's script body from the top, evaluating each
// if/else condition and applying each controller-call statement in
// sequence, the same "stop at the first state change" rule
// statemachine.Step follows: once a ChangeState-shaped statement applies,
// no further statement in the body -- at any nesting level, including
// ones after the enclosing if block -- runs for this call. ctx itself is
// never mutated; Step returns an independent, updated copy.
//
// Step returns a descriptive error, never a panic, when ctx.StateNo has no
// matching Statedef block in script, when the body contains a construct
// this package does not support (see the package doc comment), or when
// running it fails the same way statemachine.Step's own errors do (an
// unknown trigger name, a ChangeState target missing from script, ...).
func Step(ctx evaluator.Context, script zss.Script) (Result, error) {
	block, ok := findStatedef(script, ctx.StateNo)
	if !ok {
		return Result{}, fmt.Errorf("zssexec: current state %d not found in loaded .zss script", ctx.StateNo)
	}

	stmts, err := parseBody(block.Body)
	if err != nil {
		return Result{}, fmt.Errorf("zssexec: state %d: %w", ctx.StateNo, err)
	}

	working := ctx
	exists := func(n int) bool {
		_, ok := findStatedef(script, n)
		return ok
	}
	if _, err := execStmts(stmts, &working, script, exists); err != nil {
		return Result{}, fmt.Errorf("zssexec: state %d: %w", ctx.StateNo, err)
	}

	return Result{Context: working}, nil
}

// execStmts runs stmts in order against ctx, mutating it in place, and
// reports whether a state change happened -- the caller must stop running
// any further statements at its own level (and every enclosing level) as
// soon as this is true.
func execStmts(stmts []stmt, ctx *evaluator.Context, script zss.Script, exists func(int) bool) (bool, error) {
	for _, s := range stmts {
		stopped, err := execStmt(s, ctx, script, exists)
		if err != nil {
			return false, err
		}
		if stopped {
			return true, nil
		}
	}
	return false, nil
}

func execStmt(s stmt, ctx *evaluator.Context, script zss.Script, exists func(int) bool) (bool, error) {
	switch st := s.(type) {
	case ifStmt:
		v, err := evaluator.Evaluate(st.cond, *ctx)
		if err != nil {
			return false, fmt.Errorf("evaluating if condition %q: %w", st.cond, err)
		}
		branch := st.then
		if !v.Bool() {
			branch = st.els
		}
		return execStmts(branch, ctx, script, exists)

	case callStmt:
		fn, ok := findFunction(script, st.name)
		if !ok {
			return false, fmt.Errorf("call to undefined function %q", st.name)
		}
		if len(fn.Params) > 0 || len(fn.Ret) > 0 {
			return false, fmt.Errorf("call to function %q: functions with parameters or a return value are not supported yet", st.name)
		}
		fnStmts, err := parseBody(fn.Body)
		if err != nil {
			return false, fmt.Errorf("function %q: %w", st.name, err)
		}
		return execStmts(fnStmts, ctx, script, exists)

	case ctrlStmt:
		changed, err := statemachine.ApplyController(st.ctrl, ctx, exists)
		if err != nil {
			return false, fmt.Errorf("applying %s: %w", st.ctrl.Type, err)
		}
		return changed, nil

	default:
		// Unreachable: parseBody never produces any other stmt type.
		return false, fmt.Errorf("internal error: unknown statement type %T", s)
	}
}

// findStatedef returns script's Statedef block for the given state number,
// if any.
func findStatedef(script zss.Script, number int) (zss.Block, bool) {
	for _, b := range script.Blocks {
		if b.Kind == zss.BlockKindStatedef && b.Number == number {
			return b, true
		}
	}
	return zss.Block{}, false
}

// findFunction returns script's Function block with the given name, if
// any.
func findFunction(script zss.Script, name string) (zss.Block, bool) {
	for _, b := range script.Blocks {
		if b.Kind == zss.BlockKindFunction && b.Name == name {
			return b, true
		}
	}
	return zss.Block{}, false
}
