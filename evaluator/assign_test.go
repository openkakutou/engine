package evaluator

import (
	"errors"
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

func TestEvaluateMut_FloatVarAssign_ReturnsErrFloatVarUnsupported(t *testing.T) {
	for _, expr := range []string{"fvar(1) := 1.5", "SysFVar(0) := 1"} {
		_, err := EvaluateMut(expr, &Context{})
		if !errors.Is(err, ErrFloatVarUnsupported) {
			t.Errorf("%q error = %v, want ErrFloatVarUnsupported", expr, err)
		}
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

func TestLocalNames_ListsReferencedLocals_InOrderWithoutDuplicates(t *testing.T) {
	got, err := LocalNames("$b + $a * $b > var($c) && IfElse($a, $d, 1)")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"b", "a", "c", "d"}; !reflect.DeepEqual(got, want) {
		t.Errorf("LocalNames = %v, want %v", got, want)
	}
	got, err = LocalNames("var(1) := $z")
	if err != nil || !reflect.DeepEqual(got, []string{"z"}) {
		t.Errorf("LocalNames over an assignment = %v, %v, want [z]", got, err)
	}
	if got, _ := LocalNames("Time + 1"); len(got) != 0 {
		t.Errorf("LocalNames without locals = %v, want empty", got)
	}
	if _, err := LocalNames("$x := 1"); err == nil {
		t.Error("LocalNames on a parse error returned no error")
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
