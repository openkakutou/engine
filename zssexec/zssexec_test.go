package zssexec

import (
	"os"
	"strings"
	"testing"

	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/match"
)

// mustParseScript builds a zss.Script from the given block sources, one
// "[Statedef ...]"/"[Function ...]" block plus body per string, joined with
// blank lines the same way real .zss text separates blocks.
func mustParseScript(t *testing.T, blocks ...string) zss.Script {
	t.Helper()
	source := ""
	for i, b := range blocks {
		if i > 0 {
			source += "\n\n"
		}
		source += b
	}
	script, err := zss.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatalf("zss.Parse returned unexpected error: %v", err)
	}
	return script
}

func TestStep_IfConditionTrue_AppliesChangeState(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\nif Time = 0 {\n\tchangeState{value: 5;}\n}",
		"[Statedef 5]",
	)
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}, Time: 0}

	result, err := Step(ctx, script)
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

func TestStep_IfConditionFalse_ElseBranchApplied(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\nif Time = 0 {\n\tchangeState{value: 5;}\n} else {\n\tchangeState{value: 9;}\n}",
		"[Statedef 5]",
		"[Statedef 9]",
	)
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}, Time: 3}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.StateNo != 9 {
		t.Errorf("StateNo = %d, want 9 (else branch)", result.Context.StateNo)
	}
}

func TestStep_MultipleIfStatements_StopsAfterChangeStateApplies(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\nif Command = \"holdfwd\" {\n\tchangeState{value: 20;}\n}\nif Command = \"holdback\" {\n\tchangeState{value: 30;}\n}",
		"[Statedef 20]",
		"[Statedef 30]",
	)
	ctx := evaluator.Context{
		FighterState:   match.FighterState{StateNo: 0},
		ActiveCommands: map[string]bool{"holdfwd": true, "holdback": true},
	}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.StateNo != 20 {
		t.Errorf("StateNo = %d, want 20 (first ChangeState wins, second never evaluated)", result.Context.StateNo)
	}
}

func TestStep_ControllerStatement_AssignsVariable(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nvarSet{v: 3; value: 7;}")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.Vars[3] != 7 {
		t.Errorf("Vars[3] = %d, want 7", result.Context.Vars[3])
	}
	if result.Context.StateNo != 0 {
		t.Errorf("StateNo = %d, want 0 (VarSet is not a state change)", result.Context.StateNo)
	}
}

func TestStep_CallFunction_ExecutesFunctionBody(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\ncall MarkEntered();",
		"[Function MarkEntered()]\nvarSet{v: 0; value: 1;}",
	)
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.Vars[0] != 1 {
		t.Errorf("Vars[0] = %d, want 1 (set by the called function)", result.Context.Vars[0])
	}
}

func TestStep_CallUndefinedFunction_ReturnsDescriptiveError(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\ncall DoesNotExist();")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, script)
	if err == nil {
		t.Fatal("expected an error for a call to an undefined function, got nil")
	}
}

func TestStep_CallFunctionWithParameters_ReturnsDescriptiveError(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\ncall Helper();",
		"[Function Helper(x)]\nvarSet{v: 0; value: 1;}",
	)
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, script)
	if err == nil {
		t.Fatal("expected an error for a call to a function declaring parameters (not supported yet), got nil")
	}
}

func TestStep_UnsupportedConstruct_ReturnsDescriptiveError(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nlet x = 1;")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, script)
	if err == nil {
		t.Fatal("expected an error for an unsupported .zss construct (let), got nil")
	}
}

func TestStep_UnknownCurrentState_ReturnsDescriptiveError(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 999}}

	_, err := Step(ctx, script)
	if err == nil {
		t.Fatal("expected an error for a StateNo missing from the loaded script, got nil")
	}
}

func TestStep_ChangeStateTargetingNonexistentState_ReturnsDescriptiveError(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nchangeState{value: 999;}")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, script)
	if err == nil {
		t.Fatal("expected a descriptive error for a ChangeState target missing from the loaded script, got nil")
	}
}

func TestStep_ConditionEvaluationError_ReturnsDescriptiveError(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nif SomeUnknownTrigger {\n\tchangeState{value: 5;}\n}")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, script)
	if err == nil {
		t.Fatal("expected an error for an if condition referencing an unknown trigger name, got nil")
	}
}

func mustParseFixtureScript(t *testing.T, path string) zss.Script {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	script, err := zss.Parse(f)
	if err != nil {
		t.Fatalf("zss.Parse returned unexpected error: %v", err)
	}
	return script
}

// TestStep_RealCharacterFixture_KfmIdle_NoCommandHeld_CallsFunctionAndSetsVar
// loads a .zss idle-state fixture styled after the same Kung Fu Man sample
// character statemachine's own testdata/kfm_idle.cns fixture uses. With no
// movement command held, both ChangeState-guarded ifs are false, so
// execution reaches the Time = 0 branch and calls its helper function,
// confirming a nested function call's controller-style statement mutates
// the fighter's variables.
func TestStep_RealCharacterFixture_KfmIdle_NoCommandHeld_CallsFunctionAndSetsVar(t *testing.T) {
	script := mustParseFixtureScript(t, "testdata/kfm_idle.zss")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}, Time: 0}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.StateNo != 0 {
		t.Errorf("StateNo = %d, want 0 (no command held, no state change)", result.Context.StateNo)
	}
	if result.Context.Vars[0] != 1 {
		t.Errorf("Vars[0] = %d, want 1 (MarkEnteredIdle called since Time = 0)", result.Context.Vars[0])
	}
}

// TestStep_RealCharacterFixture_KfmIdle_HoldFwd_ChangesStateAndStops mirrors
// statemachine's own real-fixture behavior: once the holdfwd command's
// ChangeState applies, the later Time = 0 branch (and its function call)
// never runs for this tick, the same "stop at the first state change" rule
// .cns-driven states follow.
func TestStep_RealCharacterFixture_KfmIdle_HoldFwd_ChangesStateAndStops(t *testing.T) {
	script := mustParseFixtureScript(t, "testdata/kfm_idle.zss")
	ctx := evaluator.Context{
		FighterState:   match.FighterState{StateNo: 0},
		ActiveCommands: map[string]bool{"holdfwd": true},
	}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.StateNo != 20 {
		t.Errorf("StateNo = %d, want 20 (holdfwd command recognized)", result.Context.StateNo)
	}
	if result.Context.Vars[0] != 0 {
		t.Errorf("Vars[0] = %d, want 0 (MarkEnteredIdle never reached once ChangeState applied)", result.Context.Vars[0])
	}
}
