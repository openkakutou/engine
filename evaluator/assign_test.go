package evaluator

import (
	"reflect"
	"strings"
	"testing"
)

// Reference: Ikemen GO commit 54d4148a. `:=` is parsed right after a var /
// sysvar / fvar / sysfvar atom (src/compiler.go:1331 `set := c.token == ":="`),
// its right side by expEqne (compiler.go:5945: comparisons and everything
// tighter, not && / ||), and the expression's value is the assigned value
// (src/char.go:8333-8334 varSet returns BytecodeInt(v)).

func TestEvaluateMut_VarAssign_SetsTheSlotAndReturnsTheValue(t *testing.T) {
	ctx := &Context{}
	got, err := EvaluateMut("var(3) := 5", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Int() != 5 || ctx.Vars[3] != 5 {
		t.Errorf("value %v, Vars[3] %d, want 5 and 5", got.Int(), ctx.Vars[3])
	}
	if ctx.Vars[2] != 0 || ctx.Vars[4] != 0 {
		t.Errorf("neighbouring slots changed: %v", ctx.Vars[:6])
	}
}

func TestEvaluateMut_SysVarAssignOfABoolean_StoresOneAndReturnsOne(t *testing.T) {
	ctx := &Context{}
	got, err := EvaluateMut("sysvar(0) := (1 && 1)", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Int() != 1 || ctx.SysVars[0] != 1 {
		t.Errorf("value %v, SysVars[0] %d, want 1 and 1", got.Int(), ctx.SysVars[0])
	}
}

func TestEvaluateMut_MixedCaseSysVarAssign_Works(t *testing.T) {
	ctx := &Context{}
	ctx.SysVars[1] = 9
	if _, err := EvaluateMut("SysVar(1) := 0", ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.SysVars[1] != 0 {
		t.Errorf("SysVars[1] = %d, want 0", ctx.SysVars[1])
	}
}

func TestEvaluateMut_NestedVarIndex_AssignsTheSlotNamedByTheInnerVar(t *testing.T) {
	ctx := &Context{}
	ctx.Vars[0] = 4
	if _, err := EvaluateMut("var(var(0)) := 7", ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.Vars[4] != 7 || ctx.Vars[0] != 4 {
		t.Errorf("Vars[4]=%d Vars[0]=%d, want 7 and 4", ctx.Vars[4], ctx.Vars[0])
	}
}

func TestEvaluateMut_AssignRightSide_TakesArithmeticWhole(t *testing.T) {
	ctx := &Context{}
	got, err := EvaluateMut("var(1) := 2 + 3 * 2", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Vars[1] != 8 || got.Int() != 8 {
		t.Errorf("Vars[1]=%d value=%d, want 8 (2+3*2, not (var(1):=2)+...)", ctx.Vars[1], got.Int())
	}
}

func TestEvaluateMut_AssignRightSide_StopsBeforeLogicalAnd(t *testing.T) {
	// expEqne does not consume &&: this is (var(1) := 2) && 0, so the slot
	// holds 2 and the whole expression is false.
	ctx := &Context{}
	got, err := EvaluateMut("var(1) := 2 && 0", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Vars[1] != 2 || got.Bool() {
		t.Errorf("Vars[1]=%d bool=%v, want 2 and false", ctx.Vars[1], got.Bool())
	}
}

func TestEvaluateMut_AssignRightSide_IncludesEquality(t *testing.T) {
	// expEqne handles = : var(1) := (3 = 3) stores 1.
	ctx := &Context{}
	if _, err := EvaluateMut("var(1) := 3 = 3", ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.Vars[1] != 1 {
		t.Errorf("Vars[1] = %d, want 1", ctx.Vars[1])
	}
}

func TestEvaluateMut_AssignUsedAsOperand_FeedsTheEnclosingExpression(t *testing.T) {
	ctx := &Context{}
	got, err := EvaluateMut("(var(2) := 6) + 1", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Int() != 7 || ctx.Vars[2] != 6 {
		t.Errorf("value %d Vars[2] %d, want 7 and 6", got.Int(), ctx.Vars[2])
	}
}

func TestEvaluate_AssignByValueContext_IsAnErrorNotASilentDrop(t *testing.T) {
	if _, err := Evaluate("var(3) := 5", Context{}); err == nil {
		t.Error("Evaluate on an assignment returned no error; the write would be lost")
	}
	e, err := Parse("var(3) := 5")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Eval(Context{}); err == nil {
		t.Error("Eval on an assignment returned no error")
	}
}

func TestEvaluateMut_AssignErrors_AreReported(t *testing.T) {
	tests := []struct {
		name, expr, want string
	}{
		{"local target", "$x := 1", "local"},
		{"index too high", "var(60) := 1", "out of range"},
		{"negative index", "var(0 - 1) := 1", "out of range"},
		{"sysvar too high", "sysvar(5) := 1", "out of range"},
		{"literal target", "3 := 1", ":="},
		{"missing value", "var(1) :=", "end of expression"},
	}
	for _, tt := range tests {
		ctx := &Context{Locals: LocalFrame{Names: []string{"x"}, Values: []Value{intValue(1)}}}
		_, err := EvaluateMut(tt.expr, ctx)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%s: %q error = %v, want it to contain %q", tt.name, tt.expr, err, tt.want)
		}
	}
}

// Ikemen GO stores fvar/sysfvar as float32 (char.go cnsfvar, cnssysfvar) and
// returns the stored value from the assignment (char.go:8336-8342).
func TestEvaluateMut_FloatVarAssign_StoresFloat32AndReturnsIt(t *testing.T) {
	ctx := &Context{}
	got, err := EvaluateMut("fvar(1) := 0.1", ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := float64(float32(0.1))
	if got.Kind != KindFloat || got.Number != want || float64(ctx.FVars[1]) != want {
		t.Errorf("fvar(1) := 0.1 = %v (kind %v), FVars[1] = %v, want float32(0.1) = %v", got.Number, got.Kind, ctx.FVars[1], want)
	}
	if _, err := EvaluateMut("SysFVar(4) := 3", ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.SysFVars[4] != 3 || ctx.FVars[4] != 0 {
		t.Errorf("SysFVars[4] = %v, FVars[4] = %v, want 3 and 0", ctx.SysFVars[4], ctx.FVars[4])
	}
}

func TestEval_FVarAndSysFVar_ReadTheirSlotsAsFloats(t *testing.T) {
	var ctx Context
	ctx.FVars[39] = 2.5
	ctx.SysFVars[2] = -0.75
	cases := []struct {
		expr string
		want float64
	}{
		{"FVar(39)", 2.5},
		{"FVar(0)", 0}, // never written reads 0 (char.go:8248-8261)
		{"SysFVar(2)", -0.75},
		{"FVar(40 - 1) * 2", 5},
	}
	for _, tc := range cases {
		got := mustEval(t, tc.expr, ctx)
		if got.Number != tc.want || (tc.want != 0 && got.Kind != KindFloat) {
			t.Errorf("%s = %v (kind %v), want %v", tc.expr, got.Number, got.Kind, tc.want)
		}
	}
}

func TestEval_FloatVarIndexOutOfRange_IsAnErrorNamingTheRange(t *testing.T) {
	for expr, want := range map[string]string{
		"FVar(40)":    "fvar index 40 out of range 0-39",
		"FVar(0 - 1)": "fvar index -1 out of range 0-39",
		"SysFVar(5)":  "sysfvar index 5 out of range 0-4",
	} {
		if _, err := Evaluate(expr, Context{}); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s error = %v, want one containing %q", expr, err, want)
		}
	}
	if _, err := EvaluateMut("fvar(40) := 1", &Context{}); err == nil || !strings.Contains(err.Error(), "out of range") {
		t.Errorf("fvar(40) := 1 error = %v, want out of range", err)
	}
}

func TestContext_AssignVar_StoresEachKindAndRejectsAnUnknownTarget(t *testing.T) {
	var ctx Context
	for _, tc := range []struct {
		target string
		idx    int
		in     Value
		want   float64
	}{
		{"var", 2, floatValue(7.9), 7}, // ints truncate
		{"sysvar", 1, intValue(4), 4},
		{"fvar", 3, intValue(2), 2},
		{"sysfvar", 0, floatValue(1.5), 1.5},
	} {
		got, err := ctx.AssignVar(tc.target, tc.idx, tc.in)
		if err != nil || got.Number != tc.want {
			t.Errorf("AssignVar(%s, %d, %v) = %v, %v, want %v", tc.target, tc.idx, tc.in.Number, got.Number, err, tc.want)
		}
	}
	if ctx.Vars[2] != 7 || ctx.SysVars[1] != 4 || ctx.FVars[3] != 2 || ctx.SysFVars[0] != 1.5 {
		t.Errorf("slots = %v %v %v %v", ctx.Vars[2], ctx.SysVars[1], ctx.FVars[3], ctx.SysFVars[0])
	}
	if _, err := ctx.AssignVar("nope", 0, intValue(1)); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Errorf("unknown target error = %v, want one naming it", err)
	}
}

func TestLocals_ReadByName_ReturnsTheMatchingSlot(t *testing.T) {
	ctx := Context{Locals: LocalFrame{
		Names:  []string{"a", "x", "_n2"},
		Values: []Value{intValue(1), intValue(21), floatValue(0.5)},
	}}
	if got := mustEval(t, "$x * 2", ctx); got.Int() != 42 {
		t.Errorf("$x * 2 = %v, want 42", got.Number)
	}
	if got := mustEval(t, "$a + $x + $_n2", ctx); got.Number != 22.5 {
		t.Errorf("$a + $x + $_n2 = %v, want 22.5", got.Number)
	}
	if got := mustEval(t, "$X", ctx); got.Int() != 21 {
		t.Errorf("$X = %v, want 21 (names are lower-cased)", got.Number)
	}
}

func TestLocals_UnsetSlot_ReadsAsZero(t *testing.T) {
	// Ikemen resets frame slots to bvNone (bytecode.go:1551), whose value is 0.
	ctx := Context{Locals: LocalFrame{Names: []string{"x"}, Values: []Value{{}}}}
	if got := mustEval(t, "$x + 3", ctx); got.Int() != 3 {
		t.Errorf("$x + 3 = %v, want 3", got.Number)
	}
}

func TestLocals_UnknownName_ErrorNamesIt(t *testing.T) {
	ctx := Context{Locals: LocalFrame{Names: []string{"x"}, Values: []Value{intValue(1)}}}
	_, err := Evaluate("$y + 1", ctx)
	if err == nil || !strings.Contains(err.Error(), "$y") {
		t.Errorf("error = %v, want one naming $y", err)
	}
	_, err = Evaluate("$x", Context{})
	if err == nil {
		t.Error("$x with an empty frame returned no error")
	}
}

func TestLocals_InvalidNames_AreParseErrors(t *testing.T) {
	for _, expr := range []string{"$", "$1a", "$_"} {
		if _, err := Parse(expr); err == nil {
			t.Errorf("Parse(%q) succeeded, want an error", expr)
		}
	}
}

func TestTriggerNames_AssignmentAndLocals_ListInnerTriggersOnly(t *testing.T) {
	got, err := TriggerNames("var(Time) := $x + Life")
	if err != nil {
		t.Fatal(err)
	}
	want := []TriggerName{{Name: "time"}, {Name: "life"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TriggerNames = %v, want %v (no entry for var, :=, or $x)", got, want)
	}
}

func TestAssignAndLocals_WarmCachedPath_AllocateNothing(t *testing.T) {
	ctx := &Context{Locals: LocalFrame{Names: []string{"x"}, Values: []Value{intValue(21)}}}
	for _, expr := range []string{"var(3) := 1", "$x + 1"} {
		if _, err := EvaluateMut(expr, ctx); err != nil { // warm the parse cache
			t.Fatalf("EvaluateMut(%q): %v", expr, err)
		}
		allocs := testing.AllocsPerRun(100, func() { _, _ = EvaluateMut(expr, ctx) })
		if allocs != 0 {
			t.Errorf("EvaluateMut(%q) allocated %v times per call, want 0", expr, allocs)
		}
	}
}
