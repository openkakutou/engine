package evaluator

import (
	"testing"

	"github.com/openkakutou/engine/match"
)

func mustEval(t *testing.T, expr string, ctx Context) Value {
	t.Helper()
	e, err := Parse(expr)
	if err != nil {
		t.Fatalf("Parse(%q) returned error: %v", expr, err)
	}
	v, err := e.Eval(ctx)
	if err != nil {
		t.Fatalf("Eval(%q) returned error: %v", expr, err)
	}
	return v
}

func TestEval_StateNo_ReadsFromEmbeddedFighterState(t *testing.T) {
	ctx := Context{FighterState: match.FighterState{StateNo: 200}}

	got := mustEval(t, "StateNo = 200", ctx)
	if !got.Bool() {
		t.Errorf("StateNo = 200 evaluated to %v, want true", got.Bool())
	}

	got = mustEval(t, "StateNo = 5", ctx)
	if got.Bool() {
		t.Errorf("StateNo = 5 evaluated to %v, want false", got.Bool())
	}
}

func TestEval_Time_SupportsTheTimeEqualsZeroIdiom(t *testing.T) {
	// "!Time" is the standard MUGEN idiom for "this is the first tick a
	// fighter has spent in its current state".
	freshEntry := Context{Time: 0}
	got := mustEval(t, "!Time", freshEntry)
	if !got.Bool() {
		t.Errorf("!Time at Time=0 evaluated to %v, want true", got.Bool())
	}

	laterTick := Context{Time: 12}
	got = mustEval(t, "!Time", laterTick)
	if got.Bool() {
		t.Errorf("!Time at Time=12 evaluated to %v, want false", got.Bool())
	}
}

func TestEval_AnimAndAnimTime_ReadFromContext(t *testing.T) {
	ctx := Context{Anim: 190, AnimTime: 3}

	if got := mustEval(t, "Anim = 190", ctx); !got.Bool() {
		t.Errorf("Anim = 190 evaluated to %v, want true", got.Bool())
	}
	if got := mustEval(t, "AnimTime = 0", ctx); got.Bool() {
		t.Errorf("AnimTime = 0 evaluated to %v, want false", got.Bool())
	}
}

func TestEval_Ctrl_ReadsBooleanFlag(t *testing.T) {
	if got := mustEval(t, "Ctrl", Context{Ctrl: true}); !got.Bool() {
		t.Errorf("Ctrl (true) evaluated to %v, want true", got.Bool())
	}
	if got := mustEval(t, "Ctrl", Context{Ctrl: false}); got.Bool() {
		t.Errorf("Ctrl (false) evaluated to %v, want false", got.Bool())
	}
}

func TestEval_Command_ChecksActiveCommandSet(t *testing.T) {
	ctx := Context{ActiveCommands: map[string]bool{"holdback": true}}

	if got := mustEval(t, `Command = "holdback"`, ctx); !got.Bool() {
		t.Errorf(`Command = "holdback" evaluated to %v, want true`, got.Bool())
	}
	if got := mustEval(t, `Command = "holdfwd"`, ctx); got.Bool() {
		t.Errorf(`Command = "holdfwd" evaluated to %v, want false`, got.Bool())
	}
	if got := mustEval(t, `Command != "holdfwd"`, ctx); !got.Bool() {
		t.Errorf(`Command != "holdfwd" evaluated to %v, want true`, got.Bool())
	}
}

func TestEval_Var_ReadsIndexedVariable(t *testing.T) {
	ctx := Context{}
	ctx.Vars[0] = 5

	if got := mustEval(t, "var(0) = 5", ctx); !got.Bool() {
		t.Errorf("var(0) = 5 evaluated to %v, want true", got.Bool())
	}
	// An index never written reads as MUGEN's default of 0.
	if got := mustEval(t, "var(1) = 0", ctx); !got.Bool() {
		t.Errorf("var(1) = 0 evaluated to %v, want true", got.Bool())
	}
}

func TestEval_SysVar_ReadsIndexedSystemVariable(t *testing.T) {
	ctx := Context{}
	ctx.SysVars[0] = 1

	if got := mustEval(t, "sysvar(0) != 1", ctx); got.Bool() {
		t.Errorf("sysvar(0) != 1 evaluated to %v, want false", got.Bool())
	}
}

func TestEval_IfElse_SelectsBranchByCondition(t *testing.T) {
	got := mustEval(t, "IfElse(Ctrl, 1, 0) = 1", Context{Ctrl: true})
	if !got.Bool() {
		t.Errorf("IfElse(Ctrl, 1, 0) = 1 with Ctrl=true evaluated to %v, want true", got.Bool())
	}

	got = mustEval(t, "IfElse(Ctrl, 1, 0) = 1", Context{Ctrl: false})
	if got.Bool() {
		t.Errorf("IfElse(Ctrl, 1, 0) = 1 with Ctrl=false evaluated to %v, want false", got.Bool())
	}
}

func TestEval_ArithmeticResultKind_IsIntWhenBothOperandsAreInt(t *testing.T) {
	e, err := Parse("1 + 1")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	v, err := e.Eval(Context{})
	if err != nil {
		t.Fatalf("unexpected eval error: %v", err)
	}
	if v.Kind != KindInt {
		t.Errorf("1 + 1 Kind = %v, want KindInt", v.Kind)
	}
	if v.Number != 2 {
		t.Errorf("1 + 1 = %v, want 2", v.Number)
	}
}

func TestEval_ArithmeticResultKind_IsFloatWhenEitherOperandIsFloat(t *testing.T) {
	e, err := Parse("1 + 1.5")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	v, err := e.Eval(Context{})
	if err != nil {
		t.Fatalf("unexpected eval error: %v", err)
	}
	if v.Kind != KindFloat {
		t.Errorf("1 + 1.5 Kind = %v, want KindFloat", v.Kind)
	}
	if v.Number != 2.5 {
		t.Errorf("1 + 1.5 = %v, want 2.5", v.Number)
	}
}

func TestEval_LooseTyping_ComparesIntAndFloatByNumericValue(t *testing.T) {
	got := mustEval(t, "1 = 1.0", Context{})
	if !got.Bool() {
		t.Errorf("1 = 1.0 evaluated to %v, want true", got.Bool())
	}
	if got.Kind != KindBool {
		t.Errorf("1 = 1.0 Kind = %v, want KindBool", got.Kind)
	}
}

func TestEval_ReturnsError_OnUnknownTriggerName(t *testing.T) {
	e, err := Parse("StateType != A")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if _, err := e.Eval(Context{}); err == nil {
		t.Error("expected error for unsupported trigger name StateType, got nil")
	}
}

func TestEval_ReturnsError_OnUnknownFunctionName(t *testing.T) {
	e, err := Parse("Random(0, 999)")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if _, err := e.Eval(Context{}); err == nil {
		t.Error("expected error for unsupported function Random, got nil")
	}
}

func TestEval_ReturnsError_OnVarIndexOutOfRange(t *testing.T) {
	e, err := Parse("var(999) = 0")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if _, err := e.Eval(Context{}); err == nil {
		t.Error("expected error for out-of-range var index, got nil")
	}
}

func TestEval_ReturnsError_OnWrongArgumentCount(t *testing.T) {
	e, err := Parse("IfElse(Ctrl, 1)")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if _, err := e.Eval(Context{}); err == nil {
		t.Error("expected error for wrong IfElse argument count, got nil")
	}
}

func TestEval_ReturnsError_OnStringLiteralOutsideCommandComparison(t *testing.T) {
	e, err := Parse(`"holdback"`)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if _, err := e.Eval(Context{}); err == nil {
		t.Error("expected error for bare string literal, got nil")
	}
}

func TestEvaluate_ConvenienceFunction_ParsesAndEvaluatesInOneCall(t *testing.T) {
	got, err := Evaluate("1 + 1 = 2", Context{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Bool() {
		t.Errorf("Evaluate(1 + 1 = 2) = %v, want true", got.Bool())
	}
}

func TestEvaluate_ConvenienceFunction_PropagatesParseError(t *testing.T) {
	if _, err := Evaluate("1 +", Context{}); err == nil {
		t.Error("expected Evaluate to propagate a parse error, got nil")
	}
}
