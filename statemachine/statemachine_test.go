package statemachine

import (
	"os"
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/match"
)

// changeStateController builds a minimal ChangeState controller, optionally
// gated by a single trigger expression (an empty trigger runs
// unconditionally, matching cns.Controller's documented "nil/empty Triggers
// means unconditional" rule).
func changeStateController(trigger string, target int) cns.Controller {
	c := cns.Controller{
		Type:       "ChangeState",
		Parameters: map[string]string{"value": itoa(target)},
	}
	if trigger != "" {
		c.Triggers = []string{trigger}
	}
	return c
}

func varSetController(trigger string, index, value int) cns.Controller {
	c := cns.Controller{
		Type: "VarSet",
		Parameters: map[string]string{
			"v":     itoa(index),
			"value": itoa(value),
		},
	}
	if trigger != "" {
		c.Triggers = []string{trigger}
	}
	return c
}

func itoa(n int) string {
	// Avoids importing strconv just for test fixture construction of
	// small, always-non-negative-or-simple integers used in these tests.
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

func TestStep_ControllersWithFalseTriggers_AreSkippedInDeclaredOrder(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				changeStateController("Command = \"holddown\"", 11), // false: not held
				changeStateController("Command = \"holdfwd\"", 20),  // true: applied
				changeStateController("Command = \"holdback\"", 30), // never reached: loop stops after a ChangeState applies
			},
		},
		11: {Number: 11},
		20: {Number: 20},
		30: {Number: 30},
	}
	ctx := evaluator.Context{
		FighterState:   match.FighterState{StateNo: 0},
		ActiveCommands: map[string]bool{"holdfwd": true},
	}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}

	if want := []int{1}; !intSlicesEqual(result.Applied, want) {
		t.Errorf("Applied = %v, want %v", result.Applied, want)
	}
	if result.Context.StateNo != 20 {
		t.Errorf("StateNo = %d, want 20", result.Context.StateNo)
	}
}

func TestStep_ChangeStateController_ResetsTimeAndUpdatesStateNo(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				changeStateController("", 5), // unconditional
			},
		},
		5: {Number: 5},
	}
	ctx := evaluator.Context{
		FighterState: match.FighterState{StateNo: 0},
		Time:         42,
	}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}

	if result.Context.StateNo != 5 {
		t.Errorf("StateNo = %d, want 5", result.Context.StateNo)
	}
	if result.Context.Time != 0 {
		t.Errorf("Time = %d, want 0 (must reset on state change)", result.Context.Time)
	}
}

func TestStep_NoStateChange_LeavesTimeUntouched(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				varSetController("Time = 0", 2, 9), // false: Time is not 0 here
			},
		},
	}
	ctx := evaluator.Context{
		FighterState: match.FighterState{StateNo: 0},
		Time:         7,
	}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.Time != 7 {
		t.Errorf("Time = %d, want 7 (Step must not auto-increment Time)", result.Context.Time)
	}
}

func TestStep_UnknownCurrentState_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {Number: 0},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 999}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal("expected an error for a StateNo missing from the loaded character, got nil")
	}
}

func TestStep_ChangeStateTargetingNonexistentState_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				changeStateController("", 999), // 999 is never defined below
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal("expected a descriptive error for a ChangeState target missing from the loaded character, got nil")
	}
}

func TestStep_ChangeStateMissingValueParameter_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				{Type: "ChangeState", Parameters: map[string]string{}},
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal("expected a descriptive error for a ChangeState controller missing its \"value\" parameter, got nil")
	}
}

func TestStep_ControllerTriggerEvaluationError_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				changeStateController("SomeUnknownTrigger", 5),
			},
		},
		5: {Number: 5},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal("expected an error for a controller trigger referencing an unknown trigger name, got nil")
	}
}

func TestStep_VarSetController_AssignsVariable(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				varSetController("", 3, 7), // unconditional
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.Vars[3] != 7 {
		t.Errorf("Vars[3] = %d, want 7", result.Context.Vars[3])
	}
	if want := []int{0}; !intSlicesEqual(result.Applied, want) {
		t.Errorf("Applied = %v, want %v", result.Applied, want)
	}
}

func TestStep_VarSetMissingVParameter_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				{Type: "VarSet", Parameters: map[string]string{"value": "7"}},
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal("expected a descriptive error for a VarSet controller missing its \"v\" parameter, got nil")
	}
}

func TestStep_VarSetMissingValueParameter_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				{Type: "VarSet", Parameters: map[string]string{"v": "3"}},
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal("expected a descriptive error for a VarSet controller missing its \"value\" parameter, got nil")
	}
}

func TestStep_VarSetUnevaluableVExpression_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				{Type: "VarSet", Parameters: map[string]string{"v": "SomeUnknownTrigger", "value": "7"}},
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal("expected a descriptive error for a VarSet \"v\" expression that fails to evaluate, got nil")
	}
}

func TestStep_VarSetUnevaluableValueExpression_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				{Type: "VarSet", Parameters: map[string]string{"v": "3", "value": "SomeUnknownTrigger"}},
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal("expected a descriptive error for a VarSet \"value\" expression that fails to evaluate, got nil")
	}
}

func TestStep_VarSetIndexOutOfRange_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				varSetController("", 999, 7),
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal("expected a descriptive error for a VarSet index out of range, got nil")
	}
}

func TestStep_UnimplementedControllerType_TriggersTrueButHasNoEffect(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				{
					Type:       "VelSet", // not implemented by this item
					Triggers:   []string{"1"},
					Parameters: map[string]string{"x": "5"},
				},
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if want := []int{0}; !intSlicesEqual(result.Applied, want) {
		t.Errorf("Applied = %v, want %v (trigger evaluated true even though the effect is unimplemented)", result.Applied, want)
	}
	if result.Context.StateNo != ctx.StateNo {
		t.Errorf("StateNo = %d, want unchanged %d (unimplemented controller types must have no effect)", result.Context.StateNo, ctx.StateNo)
	}
	if result.Context.Vars != ctx.Vars {
		t.Errorf("Vars = %v, want unchanged %v (unimplemented controller types must have no effect)", result.Context.Vars, ctx.Vars)
	}
}

// The next two tests are a matched pair: the same two controllers (a
// VarSet that marks a flag, and a ChangeState gated on that flag) are
// exercised in both orders. Only when the VarSet runs first does the
// ChangeState's trigger see the flag it depends on -- proving that
// declared order, not just each trigger's own truth value in isolation,
// determines which controllers actually apply.
func TestStep_MultiControllerState_EarlierVarSetEnablesLaterChangeState(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				varSetController("", 0, 1),              // unconditional: sets var(0) = 1
				changeStateController("var(0) = 1", 20), // depends on the controller above
			},
		},
		20: {Number: 20},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if want := []int{0, 1}; !intSlicesEqual(result.Applied, want) {
		t.Errorf("Applied = %v, want %v", result.Applied, want)
	}
	if result.Context.StateNo != 20 {
		t.Errorf("StateNo = %d, want 20 (later ChangeState must observe the earlier VarSet's effect)", result.Context.StateNo)
	}
}

func TestStep_MultiControllerState_LaterVarSetDoesNotEnableEarlierChangeState(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				changeStateController("var(0) = 1", 20), // evaluated before var(0) is ever set
				varSetController("", 0, 1),              // sets var(0) = 1, too late to matter
			},
		},
		20: {Number: 20},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if want := []int{1}; !intSlicesEqual(result.Applied, want) {
		t.Errorf("Applied = %v, want %v (the ChangeState must not have applied)", result.Applied, want)
	}
	if result.Context.StateNo != 0 {
		t.Errorf("StateNo = %d, want 0 (unchanged: the ChangeState's trigger must have evaluated false)", result.Context.StateNo)
	}
}

func TestStep_ChangeStateApplying_StopsProcessingFurtherControllers(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				changeStateController("", 20), // unconditional: applies and ends this call
				varSetController("", 0, 1),    // must never run
			},
		},
		20: {Number: 20},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if want := []int{0}; !intSlicesEqual(result.Applied, want) {
		t.Errorf("Applied = %v, want %v (the controller after ChangeState must not run)", result.Applied, want)
	}
	if result.Context.Vars[0] != 0 {
		t.Errorf("Vars[0] = %d, want 0 (the VarSet after ChangeState must never have applied)", result.Context.Vars[0])
	}
}

func TestStep_RealCharacterIdleStateFixture_ProducesExpectedControllerApplications(t *testing.T) {
	f, err := os.Open("testdata/kfm_idle.cns")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer f.Close()

	stateDefs, err := cns.Parse(f)
	if err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}
	states := make(map[int]cns.StateDef, len(stateDefs))
	for _, sd := range stateDefs {
		states[sd.Number] = sd
	}
	if _, ok := states[0]; !ok {
		t.Fatal("fixture is expected to define Statedef 0 (Standing/idle)")
	}

	// The fighter is holding forward (not down): the "ChangeState to
	// Crouch" controller's trigger is false and is skipped, the
	// "ChangeState to Walk" controller's trigger is true and applies, and
	// the trailing VarSet controller must never run since a ChangeState
	// already applied.
	ctx := evaluator.Context{
		FighterState:   match.FighterState{StateNo: 0},
		Time:           5,
		ActiveCommands: map[string]bool{"holdfwd": true},
	}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}

	if want := []int{1}; !intSlicesEqual(result.Applied, want) {
		t.Errorf("Applied = %v, want %v", result.Applied, want)
	}
	if result.Context.StateNo != 20 {
		t.Errorf("StateNo = %d, want 20 (Walk)", result.Context.StateNo)
	}
	if result.Context.Time != 0 {
		t.Errorf("Time = %d, want 0", result.Context.Time)
	}
	if result.Context.Vars[0] != 0 {
		t.Errorf("Vars[0] = %d, want 0 (the trailing VarSet must never have run)", result.Context.Vars[0])
	}
}

func intSlicesEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
