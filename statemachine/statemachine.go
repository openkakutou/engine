// Package statemachine drives a fighter's state transitions by
// interpreting character/cns's StateDef/Controller data using the trigger
// evaluator (engine/evaluator) against a fighter's live evaluator.Context.
//
// Each call to Step evaluates one simulation tick of the fighter's current
// state: its controllers are checked, in declared order, against the given
// Context, and every controller whose trigger conditions evaluate true is
// applied, updating the Context as it goes so that a later controller's
// trigger can observe an earlier one's effect within the same call. This
// item intentionally supports only the small set of controller types
// needed to prove out the execution loop -- ChangeState (state transition)
// and VarSet (variable assignment) -- not full MUGEN controller-type
// coverage, which is expected to grow via later items. A controller of an
// unimplemented type is still recorded as "applied" when its trigger
// evaluates true (so callers can observe that the condition held), but has
// no effect on the Context.
//
// A controller's Triggers ("triggerN"/"triggerall" lines) are combined
// with logical AND across the whole flat list, not MUGEN's full
// group-by-number OR-of-ANDs semantics -- character/cns's Controller does
// not retain which numeric group each trigger string came from, so that
// distinction cannot be reconstructed here. See
// .vibe/decisions/003-controller-triggers-combined-with-and.md.
//
// Matching real MUGEN/Ikemen behavior, once a ChangeState controller
// applies, Step stops evaluating any further controllers in the current
// state for that call -- control has already moved to the new state.
package statemachine

import (
	"fmt"
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/evaluator"
)

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
	def, ok := states[ctx.StateNo]
	if !ok {
		return Result{}, fmt.Errorf("statemachine: current state %d not found in loaded character", ctx.StateNo)
	}

	working := ctx
	var applied []int

	for i, ctrl := range def.Controllers {
		pass, err := triggersPass(ctrl.Triggers, working)
		if err != nil {
			return Result{}, fmt.Errorf("statemachine: state %d controller %d (%s): evaluating triggers: %w", ctx.StateNo, i, ctrl.Type, err)
		}
		if !pass {
			continue
		}
		applied = append(applied, i)

		changedState, err := ApplyController(ctrl, &working, func(n int) bool { _, ok := states[n]; return ok })
		if err != nil {
			return Result{}, fmt.Errorf("statemachine: state %d controller %d (%s): %w", ctx.StateNo, i, ctrl.Type, err)
		}
		if changedState {
			break
		}
	}

	return Result{Context: working, Applied: applied}, nil
}

// triggersPass reports whether every trigger expression in triggers
// evaluates true against ctx (logical AND across the flat list -- see the
// package doc comment). A nil or empty triggers list is unconditional and
// always passes.
func triggersPass(triggers []string, ctx evaluator.Context) (bool, error) {
	for _, trig := range triggers {
		v, err := evaluator.Evaluate(trig, ctx)
		if err != nil {
			return false, err
		}
		if !v.Bool() {
			return false, nil
		}
	}
	return true, nil
}

// ApplyController applies ctrl's effect to ctx, mutating it in place. It
// reports whether the effect was a state change (ChangeState), so the
// caller can stop processing further controllers for this call. exists
// reports whether a given state number is defined in whatever the caller is
// currently executing -- used only to validate a ChangeState target.
//
// Exported (rather than kept as Step's own private helper) so `.zss`
// execution (engine/zssexec) can apply this same small set of controller
// types to a controller-shaped statement without duplicating this logic --
// see that package's own doc comment and .vibe/decisions/008 in this repo.
//
// A controller type this item does not implement is a no-op: its trigger
// having evaluated true is still visible to the caller via Result.Applied,
// but it has no effect on ctx.
func ApplyController(ctrl cns.Controller, ctx *evaluator.Context, exists func(int) bool) (bool, error) {
	switch strings.ToLower(ctrl.Type) {
	case "changestate":
		return true, applyChangeState(ctrl, ctx, exists)
	case "varset":
		return false, applyVarSet(ctrl, ctx)
	default:
		return false, nil
	}
}

func applyChangeState(ctrl cns.Controller, ctx *evaluator.Context, exists func(int) bool) error {
	raw, ok := ctrl.Parameters["value"]
	if !ok {
		return fmt.Errorf(`ChangeState is missing its required "value" parameter`)
	}
	v, err := evaluator.Evaluate(raw, *ctx)
	if err != nil {
		return fmt.Errorf("ChangeState value %q: %w", raw, err)
	}
	target := v.Int()
	if !exists(target) {
		return fmt.Errorf("ChangeState targets state %d, which does not exist in the loaded character", target)
	}

	ctx.StateNo = target
	ctx.Time = 0
	return nil
}

func applyVarSet(ctrl cns.Controller, ctx *evaluator.Context) error {
	idxRaw, ok := ctrl.Parameters["v"]
	if !ok {
		return fmt.Errorf(`VarSet is missing its required "v" parameter`)
	}
	idxVal, err := evaluator.Evaluate(idxRaw, *ctx)
	if err != nil {
		return fmt.Errorf("VarSet v %q: %w", idxRaw, err)
	}
	idx := idxVal.Int()

	valRaw, ok := ctrl.Parameters["value"]
	if !ok {
		return fmt.Errorf(`VarSet is missing its required "value" parameter`)
	}
	val, err := evaluator.Evaluate(valRaw, *ctx)
	if err != nil {
		return fmt.Errorf("VarSet value %q: %w", valRaw, err)
	}

	if idx < 0 || idx >= len(ctx.Vars) {
		return fmt.Errorf("VarSet index %d out of range 0-%d", idx, len(ctx.Vars)-1)
	}
	ctx.Vars[idx] = val.Int()
	return nil
}
