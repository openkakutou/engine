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
// needed to prove out the execution loop -- ChangeState (state transition),
// VarSet (variable assignment), and PowerAdd (power/meter gain or spend,
// clamped to [0, DefaultMaxPower]) -- not full MUGEN controller-type
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
	RegisterController(ControllerTypePowerAdd, applyPowerAdd)
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
// A controller type with no registered handler is a no-op: its trigger
// having evaluated true is still visible to the caller via Result.Applied,
// but it has no effect on ctx.
func ApplyController(ctrl cns.Controller, ctx *evaluator.Context, exists func(int) bool) (bool, error) {
	t := strings.ToLower(ctrl.Type)
	if t == strings.ToLower(ControllerTypeChangeState) {
		return true, applyChangeState(ctrl, ctx, exists)
	}
	handler, ok := controllerHandlers[t]
	if !ok {
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

func applyVarSet(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	idxRaw, ok := ctrl.Parameters["v"]
	if !ok {
		return evaluator.Context{}, fmt.Errorf(`VarSet is missing its required "v" parameter`)
	}
	idxVal, err := evaluator.Evaluate(idxRaw, ctx)
	if err != nil {
		return evaluator.Context{}, fmt.Errorf("VarSet v %q: %w", idxRaw, err)
	}
	idx := idxVal.Int()

	valRaw, ok := ctrl.Parameters["value"]
	if !ok {
		return evaluator.Context{}, fmt.Errorf(`VarSet is missing its required "value" parameter`)
	}
	val, err := evaluator.Evaluate(valRaw, ctx)
	if err != nil {
		return evaluator.Context{}, fmt.Errorf("VarSet value %q: %w", valRaw, err)
	}

	if idx < 0 || idx >= len(ctx.Vars) {
		return evaluator.Context{}, fmt.Errorf("VarSet index %d out of range 0-%d", idx, len(ctx.Vars)-1)
	}
	ctx.Vars[idx] = val.Int()
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
	v, err := evaluator.Evaluate(raw, ctx)
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
