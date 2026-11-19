package statemachine

import (
	"os"
	"reflect"
	"strings"
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

func powerAddController(trigger string, value int) cns.Controller {
	c := cns.Controller{
		Type:       "PowerAdd",
		Parameters: map[string]string{"value": itoa(value)},
	}
	if trigger != "" {
		c.Triggers = []string{trigger}
	}
	return c
}

// playSndController builds a minimal PlaySnd controller whose raw "value"
// parameter is passed through verbatim -- callers pass either a literal
// "group,sample" pair or an expression on either side, exercising both.
func playSndController(trigger, value string) cns.Controller {
	c := cns.Controller{
		Type:       "PlaySnd",
		Parameters: map[string]string{"value": value},
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

func TestStep_PowerAddController_IncrementsPower(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				powerAddController("", 30), // unconditional
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0, Power: 10}}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.Power != 40 {
		t.Errorf("Power = %d, want 40 (10 + 30)", result.Context.Power)
	}
	if want := []int{0}; !intSlicesEqual(result.Applied, want) {
		t.Errorf("Applied = %v, want %v", result.Applied, want)
	}
}

func TestStep_PowerAddController_ClampsAtDefaultMaxPower(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				powerAddController("", 1000),
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0, Power: 2500}}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.Power != DefaultMaxPower {
		t.Errorf("Power = %d, want clamped to DefaultMaxPower (%d)", result.Context.Power, DefaultMaxPower)
	}
}

func TestStep_PowerAddController_NeverGoesNegative(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				powerAddController("", -500),
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0, Power: 200}}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.Power != 0 {
		t.Errorf("Power = %d, want floored at 0 (200 - 500)", result.Context.Power)
	}
}

func TestStep_PowerAddMissingValueParameter_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				{Type: "PowerAdd", Parameters: map[string]string{}},
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal("expected a descriptive error for a PowerAdd controller missing its \"value\" parameter, got nil")
	}
}

func TestStep_PowerAddUnevaluableValueExpression_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				{Type: "PowerAdd", Parameters: map[string]string{"value": "SomeUnknownTrigger"}},
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal("expected a descriptive error for a PowerAdd \"value\" expression that fails to evaluate, got nil")
	}
}

func TestStep_PlaySndController_RecordsGroupAndSampleEvent(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				playSndController("", "1,3"), // unconditional
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	want := []evaluator.SoundEvent{{Group: 1, Sample: 3, Channel: -1}}
	if !soundEventsEqual(result.Sounds, want) {
		t.Errorf("Sounds = %v, want %v", result.Sounds, want)
	}
	if result.Context.Sounds != nil {
		t.Errorf("Context.Sounds = %v, want nil -- Step must clear it before returning; it is a per-tick event log, not persistent fighter state", result.Context.Sounds)
	}
}

func TestStep_PlaySndController_SupportsExpressionValues(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				playSndController("", "var(0), var(0)+1"),
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}
	ctx.Vars[0] = 2

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	want := []evaluator.SoundEvent{{Group: 2, Sample: 3, Channel: -1}}
	if !soundEventsEqual(result.Sounds, want) {
		t.Errorf("Sounds = %v, want %v (value halves must be evaluated, not treated as literals)", result.Sounds, want)
	}
}

func TestStep_MultiplePlaySndControllers_RecordAllEventsInDeclaredOrder(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				playSndController("", "1,0"),
				playSndController("", "2,5"),
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	want := []evaluator.SoundEvent{{Group: 1, Sample: 0, Channel: -1}, {Group: 2, Sample: 5, Channel: -1}}
	if !soundEventsEqual(result.Sounds, want) {
		t.Errorf("Sounds = %v, want %v in declared order", result.Sounds, want)
	}
}

func TestStep_PlaySndController_FalseTrigger_RecordsNoEvent(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				playSndController("Time = 5", "1,0"), // Time starts at 0: false this call
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if len(result.Sounds) != 0 {
		t.Errorf("Sounds = %v, want none -- the trigger was false", result.Sounds)
	}
}

func TestStep_PlaySndSounds_DoNotLeakAcrossTicks(t *testing.T) {
	firing := map[int]cns.StateDef{
		0: {Number: 0, Controllers: []cns.Controller{playSndController("", "1,0")}},
	}
	silent := map[int]cns.StateDef{
		0: {Number: 0, Controllers: []cns.Controller{}},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	first, err := Step(ctx, firing)
	if err != nil {
		t.Fatalf("first Step returned unexpected error: %v", err)
	}
	if len(first.Sounds) != 1 {
		t.Fatalf("first Step: Sounds = %v, want exactly 1 event", first.Sounds)
	}

	second, err := Step(first.Context, silent)
	if err != nil {
		t.Fatalf("second Step returned unexpected error: %v", err)
	}
	if len(second.Sounds) != 0 {
		t.Errorf("second Step: Sounds = %v, want none -- must not carry over from the previous tick's Context", second.Sounds)
	}
}

func TestStep_PlaySndMissingValueParameter_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				{Type: "PlaySnd", Parameters: map[string]string{}},
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal(`expected a descriptive error for a PlaySnd controller missing its "value" parameter, got nil`)
	}
}

func TestStep_PlaySndValueMissingComma_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				playSndController("", "1"),
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal(`expected a descriptive error for a PlaySnd "value" missing its "group,sample" comma, got nil`)
	}
}

func TestStep_PlaySndUnevaluableGroupExpression_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				playSndController("", "SomeUnknownTrigger,0"),
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal(`expected a descriptive error for a PlaySnd "value" group expression that fails to evaluate, got nil`)
	}
}

func TestStep_PlaySndUnevaluableSampleExpression_ReturnsDescriptiveError(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				playSndController("", "0,SomeUnknownTrigger"),
			},
		},
	}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, states)
	if err == nil {
		t.Fatal(`expected a descriptive error for a PlaySnd "value" sample expression that fails to evaluate, got nil`)
	}
}

func TestStep_UnimplementedControllerType_TriggersTrueButHasNoEffect(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0,
			Controllers: []cns.Controller{
				{
					Type:       "Explod", // not implemented yet
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

// This test proves ApplyController's dispatch is registry-based, not a
// hardcoded switch: a controller type this package has never heard of
// ("TestOnlyController") is made dispatchable purely by calling
// RegisterController -- no case is added anywhere for it. A hardcoded
// switch could not pass this test no matter how it were extended, since
// the type name only exists inside the test.
func TestRegisterController_NewControllerType_IsDispatchedWithoutEditingApplyController(t *testing.T) {
	const controllerType = "TestOnlyController"
	called := false
	RegisterController(controllerType, func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
		called = true
		return ctx, nil
	})

	ctrl := cns.Controller{Type: controllerType}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	changedState, err := ApplyController(ctrl, &ctx, func(int) bool { return true })
	if err != nil {
		t.Fatalf("ApplyController returned unexpected error: %v", err)
	}
	if changedState {
		t.Errorf("changedState = true, want false")
	}
	if !called {
		t.Errorf("handler registered for %q was never invoked by ApplyController", controllerType)
	}
}

// RegisterController must match case-insensitively, the same way the
// three built-in controller types (ChangeState/VarSet/PowerAdd) already
// do -- a real .cns file's Type casing is not guaranteed.
func TestRegisterController_MatchesControllerTypeCaseInsensitively(t *testing.T) {
	const controllerType = "AnotherTestOnlyController"
	called := false
	RegisterController(controllerType, func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
		called = true
		return ctx, nil
	})

	ctrl := cns.Controller{Type: "anothertestonlycontroller"}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	if _, err := ApplyController(ctrl, &ctx, func(int) bool { return true }); err != nil {
		t.Fatalf("ApplyController returned unexpected error: %v", err)
	}
	if !called {
		t.Errorf("handler registered for %q was not dispatched for differently-cased type %q", controllerType, ctrl.Type)
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

func soundEventsEqual(a, b []evaluator.SoundEvent) bool {
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

func TestIsSupportedController(t *testing.T) {
	tests := []struct {
		typ  string
		want bool
	}{
		{"VarSet", true},
		{"varset", true},
		{"VARSET", true},
		{"PlaySnd", true},
		{"ChangeState", true},
		{"changestate", true},
		{"InventedController", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := IsSupportedController(tc.typ); got != tc.want {
			t.Errorf("IsSupportedController(%q) = %v, want %v", tc.typ, got, tc.want)
		}
	}
}

func TestSplitPlaySndHalves_SplitsAtFirstComma(t *testing.T) {
	got, err := SplitPlaySndValue(" 0 , Random % 3 ")
	if err != nil {
		t.Fatalf("SplitPlaySndValue error: %v", err)
	}
	if got != [2]string{"0", "Random % 3"} {
		t.Errorf("got %q", got)
	}
	if _, err := SplitPlaySndValue("5"); err == nil {
		t.Error("expected error for a value with no comma")
	}
}

func TestStepState_RunsTheNamedStateWithoutChangingCurrentState(t *testing.T) {
	states := map[int]cns.StateDef{
		-1: {Number: -1, Controllers: []cns.Controller{{Type: "VarSet", Triggers: []string{"1"}, Parameters: map[string]string{"v": "3", "value": "8"}}}},
		0:  {Number: 0},
	}
	res, err := StepState(evaluator.Context{}, -1, states)
	if err != nil {
		t.Fatalf("StepState: %v", err)
	}
	if res.Context.Vars[3] != 8 || res.Context.StateNo != 0 {
		t.Errorf("var(3)=%d StateNo=%d, want 8 and 0", res.Context.Vars[3], res.Context.StateNo)
	}
}

func TestStepState_MissingState_ReturnsErrorNamingIt(t *testing.T) {
	_, err := StepState(evaluator.Context{}, -2, map[int]cns.StateDef{0: {Number: 0}})
	if err == nil || !strings.Contains(err.Error(), "state -2") {
		t.Fatalf("err = %v, want one naming state -2", err)
	}
}

func flaggedVarSet(flag, trigger string, index, value int) cns.Controller {
	c := varSetController(trigger, index, value)
	c.Parameters["ignorehitpause"] = flag
	return c
}

func TestStepStateDuringHitPause_AppliesOnlyFlaggedControllers(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {Number: 0, Controllers: []cns.Controller{
			varSetController("", 1, 10),    // 0: unflagged
			flaggedVarSet("1", "", 2, 20),  // 1: flagged
			flaggedVarSet("0", "", 3, 30),  // 2: flag false
			flaggedVarSet("1", "0", 4, 40), // 3: flagged, trigger false
			flaggedVarSet("1", "1", 5, 50), // 4: flagged, trigger true
		}},
	}
	got, err := StepStateDuringHitPause(evaluator.Context{FighterState: match.FighterState{StateNo: 0}}, 0, states)
	if err != nil {
		t.Fatalf("StepStateDuringHitPause: %v", err)
	}
	if want := []int{1, 4}; !reflect.DeepEqual(got.Applied, want) {
		t.Errorf("Applied = %v, want %v", got.Applied, want)
	}
	if got.Context.Vars[1] != 0 || got.Context.Vars[2] != 20 || got.Context.Vars[5] != 50 {
		t.Errorf("Vars[1,2,5] = %d, %d, %d, want 0, 20, 50", got.Context.Vars[1], got.Context.Vars[2], got.Context.Vars[5])
	}
}

func TestStepStateDuringHitPause_InvalidIgnoreHitPauseExpression_ReturnsErrorNamingController(t *testing.T) {
	states := map[int]cns.StateDef{
		7: {Number: 7, Controllers: []cns.Controller{
			varSetController("", 1, 1),
			flaggedVarSet("nosuchtrigger +", "", 2, 2),
		}},
	}
	_, err := StepStateDuringHitPause(evaluator.Context{}, 7, states)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	for _, want := range []string{"state 7", "controller 1", "VarSet", "ignorehitpause"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
}

func TestApplyController_ChangeState_ZeroesPersistentCounters(t *testing.T) {
	ctx := evaluator.Context{}
	ctx.Persistent[2] = 5
	_, err := ApplyController(changeStateController("1", 30), &ctx, func(int) bool { return true })
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Persistent != [evaluator.MaxPersistent]int32{} {
		t.Errorf("Persistent = %v, want all zero after a state change", ctx.Persistent)
	}
}

// A .cns controller evaluates its parameters without short-circuit (Ikemen
// GO's CNS compiler), so the right side of && still draws Random.
func TestApplyController_CNSParameters_DoNotShortCircuit(t *testing.T) {
	ctx := evaluator.Context{RandSeed: 1}
	ctrl := cns.Controller{Type: "VelSet", Parameters: map[string]string{"x": "ifElse(time > 0 && random < 500, 1, 2)"}}
	if _, err := ApplyController(ctrl, &ctx, func(int) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if ctx.RandSeed == 1 {
		t.Error("RandSeed unchanged: the && right side was skipped in a .cns parameter")
	}
}

// Ikemen GO changes state in one table shared by .cns and .zss states
// (src/char.go changeState looks up sys.cgi[pn].states), so a ChangeState of
// a .cns state may target a state the .cns map does not hold.
func TestStepStateWith_ChangeStateToAStateKnownOnlyToExists_Applies(t *testing.T) {
	states := map[int]cns.StateDef{0: {Number: 0, Controllers: []cns.Controller{changeStateController("", 700)}}}
	exists := func(n int) bool { return n == 700 }
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	got, err := StepStateWith(ctx, 0, states, exists)
	if err != nil {
		t.Fatalf("StepStateWith: %v", err)
	}
	if got.Context.StateNo != 700 {
		t.Errorf("StateNo = %d, want 700", got.Context.StateNo)
	}
}

func TestStepStateWith_ChangeStateToAStateExistsDoesNotKnow_ReturnsError(t *testing.T) {
	states := map[int]cns.StateDef{
		0:   {Number: 0, Controllers: []cns.Controller{changeStateController("", 700)}},
		700: {Number: 700}, // in the .cns map, but the merged table says no
	}
	_, err := StepStateWith(evaluator.Context{}, 0, states, func(int) bool { return false })
	if err == nil || !strings.Contains(err.Error(), "700") {
		t.Fatalf("err = %v, want one naming state 700", err)
	}
}

func TestStepStateDuringHitPauseWith_FlaggedChangeStateToAStateKnownOnlyToExists_Applies(t *testing.T) {
	states := map[int]cns.StateDef{0: {Number: 0, Controllers: []cns.Controller{
		func() cns.Controller {
			c := changeStateController("", 700)
			c.Parameters["ignorehitpause"] = "1"
			return c
		}(),
	}}}
	got, err := StepStateDuringHitPauseWith(evaluator.Context{}, 0, states, func(n int) bool { return n == 700 })
	if err != nil {
		t.Fatalf("StepStateDuringHitPauseWith: %v", err)
	}
	if got.Context.StateNo != 700 {
		t.Errorf("StateNo = %d, want 700", got.Context.StateNo)
	}
}

// Ikemen GO (commit 54d4148a) strips an F, S or M prefix off PlaySnd's value
// when the letters right after it are empty or a known trigger
// (CharCompiler.getDataPrefix, src/compiler.go:6425-6476, called with
// ffxDefault false by playSnd, src/compiler_functions.go:363), then compiles
// the rest as the group and sample expressions. Char.playSound reads "" and
// "s" from the character's own sounds and "f" from fight.def's common.snd
// (src/char.go:6653-6663, src/fightscreen.go:4978 and 5401).
func TestStep_PlaySndController_PrefixSelectsTheSoundFile(t *testing.T) {
	cases := []struct {
		value string
		want  evaluator.SoundEvent
	}{
		{"5, 0", evaluator.SoundEvent{Group: 5, Sample: 0, Channel: -1}},
		{"S5, 0", evaluator.SoundEvent{Group: 5, Sample: 0, Channel: -1}},
		{"s 5,1", evaluator.SoundEvent{Group: 5, Sample: 1, Channel: -1}},
		{"F5, 2", evaluator.SoundEvent{Group: 5, Sample: 2, Common: true, Channel: -1}},
		{"f 5 , 3", evaluator.SoundEvent{Group: 5, Sample: 3, Common: true, Channel: -1}},
		{"F var(0)+1, 2", evaluator.SoundEvent{Group: 8, Sample: 2, Common: true, Channel: -1}},
		// "var" is a known trigger, so getDataPrefix strips the F here too.
		{"Fvar(0), 4", evaluator.SoundEvent{Group: 7, Sample: 4, Common: true, Channel: -1}},
		// "tateNo" is no trigger: StateNo is an expression, not an S prefix.
		{"StateNo, 1", evaluator.SoundEvent{Group: 0, Sample: 1, Channel: -1}},
	}
	for _, tc := range cases {
		states := map[int]cns.StateDef{0: {Number: 0, Controllers: []cns.Controller{playSndController("", tc.value)}}}
		ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}
		ctx.Vars[0] = 7
		result, err := Step(ctx, states)
		if err != nil {
			t.Errorf("value %q: Step returned unexpected error: %v", tc.value, err)
			continue
		}
		if want := []evaluator.SoundEvent{tc.want}; !soundEventsEqual(result.Sounds, want) {
			t.Errorf("value %q: Sounds = %v, want %v", tc.value, result.Sounds, want)
		}
	}
}

// The M prefix reads sys.ffx["m"] (src/char.go:6660), which nothing sets at
// commit 54d4148a: Ikemen GO plays no sound, so no event is recorded.
func TestStep_PlaySndController_MPrefix_RecordsNoEvent(t *testing.T) {
	states := map[int]cns.StateDef{0: {Number: 0, Controllers: []cns.Controller{playSndController("", "M5, 0")}}}
	result, err := Step(evaluator.Context{FighterState: match.FighterState{StateNo: 0}}, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if len(result.Sounds) != 0 {
		t.Errorf("Sounds = %v, want none", result.Sounds)
	}
}

// playSnd.Run (src/bytecode.go:5679-5685) reads channel and lowpriority;
// the default channel is -1 (newPlaySndParams, src/sound.go:1468-1476).
// The other parameters are accepted without error.
func TestStep_PlaySndController_RecordsChannelAndLowPriority(t *testing.T) {
	ctrl := playSndController("", "S0, 0")
	ctrl.Parameters["channel"] = "var(0) - 5"
	ctrl.Parameters["lowpriority"] = "1"
	ctrl.Parameters["volume"] = "-40"
	ctrl.Parameters["freqmul"] = "1.5"
	ctrl.Parameters["loop"] = "0"
	states := map[int]cns.StateDef{0: {Number: 0, Controllers: []cns.Controller{ctrl}}}
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}
	ctx.Vars[0] = 7
	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	want := []evaluator.SoundEvent{{Group: 0, Sample: 0, Channel: 2, LowPriority: true}}
	if !soundEventsEqual(result.Sounds, want) {
		t.Errorf("Sounds = %v, want %v", result.Sounds, want)
	}
}

// stepOne runs a single-controller state 0 and reports whether the
// controller applied, with the resulting Context.
func stepOne(t *testing.T, ctrl cns.Controller, ctx evaluator.Context) (bool, evaluator.Context) {
	t.Helper()
	states := map[int]cns.StateDef{0: {Number: 0, Controllers: []cns.Controller{ctrl}}}
	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	return len(result.Applied) == 1, result.Context
}

func TestStep_TriggerGroups_FireWhenTriggerAllHoldsAndAnyGroupHolds(t *testing.T) {
	ctrl := cns.Controller{
		Type:          "VarSet",
		Parameters:    map[string]string{"v": "1", "value": "7"},
		TriggerAll:    []string{"var(5) = 1"},
		TriggerGroups: [][]string{{"var(2) = 1", "var(3) = 1"}, {"var(4) = 1"}},
		Triggers:      []string{"var(5) = 1", "var(2) = 1", "var(3) = 1", "var(4) = 1"},
	}
	cases := []struct {
		name string
		vars map[int]int
		want bool
	}{
		{"only group 2 holds", map[int]int{5: 1, 4: 1}, true},
		{"only group 1 holds", map[int]int{5: 1, 2: 1, 3: 1}, true},
		{"group 1 half holds, group 2 false", map[int]int{5: 1, 2: 1}, false},
		{"groups hold but triggerall false", map[int]int{2: 1, 3: 1, 4: 1}, false},
	}
	for _, tc := range cases {
		ctx := evaluator.Context{}
		for i, v := range tc.vars {
			ctx.Vars[i] = v
		}
		if got, _ := stepOne(t, ctrl, ctx); got != tc.want {
			t.Errorf("%s: applied = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Ikemen GO compiles groups as "group1 jnz group2 ...": the first true group
// ends evaluation (src/compiler.go:7488-7493, commit 54d4148a), so a later
// group's side effects never run.
func TestStep_TriggerGroups_StopAtFirstTrueGroup_LaterAssignAndRandomNotRun(t *testing.T) {
	ctrl := cns.Controller{
		Type:          "Null",
		TriggerGroups: [][]string{{"1"}, {"(var(0) := 9) || Random >= 0"}},
		Triggers:      []string{"1", "(var(0) := 9) || Random >= 0"},
	}
	applied, out := stepOne(t, ctrl, evaluator.Context{RandSeed: 42})
	if !applied {
		t.Fatal("controller did not apply")
	}
	if out.Vars[0] != 0 {
		t.Errorf("Vars[0] = %d, want 0 (group 2 must not be evaluated)", out.Vars[0])
	}
	if out.RandSeed != 42 {
		t.Errorf("RandSeed = %d, want 42 (Random must not be drawn)", out.RandSeed)
	}
}

func TestStep_TriggerAllFalse_SkipsGroupsEntirely(t *testing.T) {
	ctrl := cns.Controller{
		Type:          "Null",
		TriggerAll:    []string{"0"},
		TriggerGroups: [][]string{{"var(0) := 9"}},
		Triggers:      []string{"0", "var(0) := 9"},
	}
	applied, out := stepOne(t, ctrl, evaluator.Context{})
	if applied || out.Vars[0] != 0 {
		t.Errorf("applied = %v, Vars[0] = %d, want false and 0", applied, out.Vars[0])
	}
}

// Ikemen GO refuses a controller without trigger1 ("Missing trigger1",
// src/compiler.go:7439-7441); character drops groups from the first gap, so
// only triggerall (or trigger2 alone) leaves TriggerGroups empty.
func TestStep_TriggersWithoutTrigger1_NeverFire(t *testing.T) {
	ctrl := cns.Controller{
		Type:       "Null",
		TriggerAll: []string{"1"},
		Triggers:   []string{"1", "1"},
	}
	if applied, _ := stepOne(t, ctrl, evaluator.Context{}); applied {
		t.Error("controller without trigger1 applied, want never")
	}
}

func TestStep_LegacyFlatTriggersOnly_StillANDed(t *testing.T) {
	ctrl := cns.Controller{Type: "Null", Triggers: []string{"1", "var(0) = 1"}}
	if applied, _ := stepOne(t, ctrl, evaluator.Context{}); applied {
		t.Error("flat-only controller applied with one false trigger, want AND")
	}
	ctx := evaluator.Context{}
	ctx.Vars[0] = 1
	if applied, _ := stepOne(t, ctrl, ctx); !applied {
		t.Error("flat-only controller did not apply with every trigger true")
	}
}

func parseOneController(t *testing.T, src string) cns.Controller {
	t.Helper()
	defs, err := cns.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatalf("cns.Parse: %v", err)
	}
	return defs[0].Controllers[0]
}

// common1.cns:128-133, verbatim: the walk ChangeAnim fires through trigger1
// while trigger2 is false.
func TestStep_RealCommon1WalkChangeAnim_FiresThroughTrigger1(t *testing.T) {
	ctrl := parseOneController(t, `[Statedef 20]
[State 20, 3]
type = ChangeAnim
triggerall = vel x > 0
trigger1 = Anim != 20 && Anim != 5
trigger2 = Anim = 5 && AnimTime = 0
value = 20
`)
	ctx := evaluator.Context{}
	ctx.Velocity.X = 2.4
	applied, out := stepOne(t, ctrl, ctx)
	if !applied || out.Anim != 20 {
		t.Errorf("applied = %v, Anim = %d, want true and 20", applied, out.Anim)
	}
}

// kfm.cmd:630-637, verbatim: during state 200 after time 6, trigger2 lets x
// chain the punch although ctrl is off.
func TestStep_RealKfmCmdStandLightPunch_FiresThroughTrigger2(t *testing.T) {
	ctrl := parseOneController(t, `[Statedef 0]
[State -1, Stand Light Punch]
type = ChangeState
value = 200
triggerall = command = "x"
triggerall = command != "holddown"
trigger1 = statetype = S
trigger1 = ctrl
trigger2 = stateno = 200
trigger2 = time > 6
`)
	states := map[int]cns.StateDef{200: {Number: 200, Controllers: []cns.Controller{ctrl}}}
	ctx := evaluator.Context{
		FighterState:   match.FighterState{StateNo: 200},
		Time:           7,
		StateType:      'S',
		ActiveCommands: map[string]bool{"x": true},
	}
	result, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if len(result.Applied) != 1 || result.Context.Time != 0 {
		t.Errorf("Applied = %v, Time = %d, want [0] and a re-entry of state 200", result.Applied, result.Context.Time)
	}
}
