package evaluator

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/openkakutou/engine/v2/match"
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
	ctx := Context{Anim: 190, AnimElapsed: 3}

	if got := mustEval(t, "Anim = 190", ctx); !got.Bool() {
		t.Errorf("Anim = 190 evaluated to %v, want true", got.Bool())
	}
	// No animation table: Ikemen GO's nil c.anim reads 0 (char.go:5604-5609),
	// whatever the elapsed counter holds.
	if got := mustEval(t, "AnimTime = 0", ctx); !got.Bool() {
		t.Errorf("AnimTime = 0 without an animation evaluated to %v, want true", got.Bool())
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

func TestEval_Comparisons_EvaluateAgainstBothSidesOfTheBoundary(t *testing.T) {
	cases := []struct {
		expr string
		want bool
	}{
		{"1 < 2", true}, {"2 < 1", false}, {"1 < 1", false},
		{"2 > 1", true}, {"1 > 2", false}, {"1 > 1", false},
		{"1 <= 1", true}, {"2 <= 1", false}, {"1 <= 2", true},
		{"1 >= 1", true}, {"1 >= 2", false}, {"2 >= 1", true},
	}
	for _, c := range cases {
		if got := mustEval(t, c.expr, Context{}); got.Bool() != c.want {
			t.Errorf("%s evaluated to %v, want %v", c.expr, got.Bool(), c.want)
		}
	}
}

func TestEval_Arithmetic_Subtraction(t *testing.T) {
	got := mustEval(t, "5 - 2 = 3", Context{})
	if !got.Bool() {
		t.Errorf("5 - 2 = 3 evaluated to %v, want true", got.Bool())
	}
}

func TestEval_Arithmetic_Division(t *testing.T) {
	got := mustEval(t, "6 / 2 = 3", Context{})
	if !got.Bool() {
		t.Errorf("6 / 2 = 3 evaluated to %v, want true", got.Bool())
	}
}

func TestEval_Arithmetic_DivisionByZero_ReturnsError(t *testing.T) {
	e, err := Parse("1 / 0")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if _, err := e.Eval(Context{}); err == nil {
		t.Error("expected error for division by zero, got nil")
	}
}

func TestEval_Arithmetic_Modulo(t *testing.T) {
	got := mustEval(t, "7 % 3 = 1", Context{})
	if !got.Bool() {
		t.Errorf("7 %% 3 = 1 evaluated to %v, want true", got.Bool())
	}
}

func TestEval_Arithmetic_ModuloByZero_ReturnsError(t *testing.T) {
	e, err := Parse("1 % 0")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if _, err := e.Eval(Context{}); err == nil {
		t.Error("expected error for modulo by zero, got nil")
	}
}

func TestEval_UnaryMinus_NegatesTheOperandAndPreservesKind(t *testing.T) {
	e, err := Parse("-5")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	v, err := e.Eval(Context{})
	if err != nil {
		t.Fatalf("unexpected eval error: %v", err)
	}
	if v.Number != -5 {
		t.Errorf("-5 = %v, want -5", v.Number)
	}
	if v.Kind != KindInt {
		t.Errorf("-5 Kind = %v, want KindInt", v.Kind)
	}

	e, err = Parse("-1.5")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	v, err = e.Eval(Context{})
	if err != nil {
		t.Fatalf("unexpected eval error: %v", err)
	}
	if v.Number != -1.5 {
		t.Errorf("-1.5 = %v, want -1.5", v.Number)
	}
	if v.Kind != KindFloat {
		t.Errorf("-1.5 Kind = %v, want KindFloat", v.Kind)
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
	e, err := Parse("BackEdgeBodyDist = 100")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if _, err := e.Eval(Context{}); err == nil {
		t.Error("expected error for unsupported trigger name BackEdgeBodyDist, got nil")
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

func TestEvaluate_ConvenienceFunction_CachedParseStillEvaluatesEachCallAgainstItsOwnContext(t *testing.T) {
	// Same expr string, evaluated repeatedly (as a cached-parse Expression
	// would be reused across simulation ticks) against different Context
	// values -- the cache must key on the parsed AST, not memoize a stale
	// result, or every later fighter/tick sharing this expr string would
	// silently see the first call's answer.
	const expr = "Time = 5"

	got, err := Evaluate(expr, Context{Time: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Bool() {
		t.Errorf("Evaluate(%q, Time=5) = %v, want true", expr, got.Bool())
	}

	got, err = Evaluate(expr, Context{Time: 9})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Bool() {
		t.Errorf("Evaluate(%q, Time=9) = %v, want false", expr, got.Bool())
	}
}

func TestEvaluate_ConvenienceFunction_PropagatesParseError(t *testing.T) {
	if _, err := Evaluate("1 +", Context{}); err == nil {
		t.Error("expected Evaluate to propagate a parse error, got nil")
	}
}

func TestEval_HitShakeOver_IsTrueOnlyWhenTheShakeTimeIsZero(t *testing.T) {
	shaking := Context{GetHit: GetHit{Active: true, HitShakeTime: 3}}
	if mustEval(t, "HitShakeOver", shaking).Bool() {
		t.Errorf("HitShakeOver with hitshaketime 3 = true, want false")
	}
	done := Context{GetHit: GetHit{Active: true, HitShakeTime: 0}}
	if !mustEval(t, "HitShakeOver", done).Bool() {
		t.Errorf("HitShakeOver with hitshaketime 0 = false, want true")
	}
}

func TestEval_HitOver_IsTrueOnlyWhenTheHitTimeIsNegativeOrNoHitWasTaken(t *testing.T) {
	cases := []struct {
		name string
		ctx  Context
		want bool
	}{
		{"hittime 0 is not over", Context{GetHit: GetHit{Active: true, HitTime: 0}}, false},
		{"hittime 5 is not over", Context{GetHit: GetHit{Active: true, HitTime: 5}}, false},
		{"hittime -1 is over", Context{GetHit: GetHit{Active: true, HitTime: -1}}, true},
		{"a fighter that never took a hit is over", Context{}, true},
	}
	for _, c := range cases {
		if got := mustEval(t, "HitOver", c.ctx).Bool(); got != c.want {
			t.Errorf("%s: HitOver = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEval_GetHitVar_ReadsTheNamedFieldWithoutEvaluatingTheName(t *testing.T) {
	ctx := Context{
		// "hittime" etc. are also not trigger names: the argument is a
		// name, never an expression (like Command's).
		GetHit: GetHit{Active: true, HitShakeTime: 8, HitTime: 11, SlideTime: 5, AnimType: 2, GroundType: 1, AirType: 3},
	}
	cases := []struct {
		expr string
		want int
	}{
		{"GetHitVar(hitshaketime)", 8},
		{"GetHitVar(hittime)", 11},
		{"GetHitVar(slidetime)", 5},
		{"GetHitVar(animtype)", 2},
		{"GetHitVar(groundtype)", 1},
		{"GetHitVar(airtype)", 3},
		{"getHitVar(AnimType)", 2},
	}
	for _, c := range cases {
		if got := mustEval(t, c.expr, ctx).Int(); got != c.want {
			t.Errorf("%s = %d, want %d", c.expr, got, c.want)
		}
	}
}

func TestEval_HitShakeOver_IsTrueForANegativeShakeTime(t *testing.T) {
	if !mustEval(t, "HitShakeOver", Context{GetHit: GetHit{Active: true, HitShakeTime: -1}}).Bool() {
		t.Errorf("HitShakeOver with hitshaketime -1 = false, want true (never stuck shaking)")
	}
}

func TestEval_GetHitVar_RejectsAnUnknownNameOrAWrongArgumentShape(t *testing.T) {
	cases := []struct {
		expr    string
		wantErr string
	}{
		{"GetHitVar(bogus)", `GetHitVar: unknown variable "bogus"`},
		{"GetHitVar(1)", "GetHitVar() expects a variable name"},
		{"GetHitVar()", "GetHitVar() expects 1 argument, got 0"},
		{"GetHitVar(hittime, slidetime)", "GetHitVar() expects 1 argument, got 2"},
		{"GetHitVar(Time)+GetHitVar(nope)", `GetHitVar: unknown variable "time"`},
	}
	for _, c := range cases {
		e, err := Parse(c.expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.expr, err)
		}
		_, err = e.Eval(Context{})
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("Eval(%q) error = %v, want one containing %q", c.expr, err, c.wantErr)
		}
	}
}

func TestEvaluate_CapitalizedGetHitTriggers_AllocateNothingOnTheCachedPath(t *testing.T) {
	ctx := Context{GetHit: GetHit{Active: true, HitTime: 3}}
	for _, expr := range []string{"HitShakeOver", "HitOver", "GetHitVar(HitTime)", "Time = 0 && Ctrl"} {
		if _, err := Evaluate(expr, ctx); err != nil { // warm the parse cache
			t.Fatalf("Evaluate(%q): %v", expr, err)
		}
		allocs := testing.AllocsPerRun(100, func() { _, _ = Evaluate(expr, ctx) })
		if allocs != 0 {
			t.Errorf("Evaluate(%q) allocated %v times per call, want 0", expr, allocs)
		}
	}
}

func TestEval_UnknownIdentifier_ReturnsUnknownTriggerError(t *testing.T) {
	_, err := Evaluate("Foo = 1", Context{})
	var ute *UnknownTriggerError
	if !errors.As(err, &ute) {
		t.Fatalf("error %v is not an *UnknownTriggerError", err)
	}
	if ute.Name != "foo" || ute.Call {
		t.Errorf("got Name=%q Call=%v, want Name=foo Call=false", ute.Name, ute.Call)
	}
	if err.Error() != `evaluator: unknown trigger "foo"` {
		t.Errorf("message = %q", err.Error())
	}
}

func TestEval_UnknownCall_ReturnsUnknownTriggerError(t *testing.T) {
	_, err := Evaluate("Bar(1) = 1", Context{})
	var ute *UnknownTriggerError
	if !errors.As(err, &ute) {
		t.Fatalf("error %v is not an *UnknownTriggerError", err)
	}
	if ute.Name != "bar" || !ute.Call {
		t.Errorf("got Name=%q Call=%v, want Name=bar Call=true", ute.Name, ute.Call)
	}
	if err.Error() != `evaluator: unknown trigger function "bar"` {
		t.Errorf("message = %q", err.Error())
	}
}

func TestTriggerNames(t *testing.T) {
	tests := []struct {
		expr string
		want []TriggerName
	}{
		{"Time = 5 && var(3) > 1", []TriggerName{{"time", false}, {"var", true}}},
		{`Command = "holdback"`, []TriggerName{{"command", false}}},
		{`"holdback" != Command && Ctrl`, []TriggerName{{"command", false}, {"ctrl", false}}},
		{"IfElse(Ctrl, IfElse(Anim = 1, Time, StateNo), -AnimTime)", []TriggerName{
			{"ifelse", true}, {"ctrl", false}, {"anim", false}, {"time", false}, {"stateno", false}, {"animtime", false}}},
		{"Time + TIME + var(0) + Var(1)", []TriggerName{{"time", false}, {"var", true}}},
		{"Time + Time(1)", []TriggerName{{"time", false}, {"time", true}}},
		{"GetHitVar(hittime) > 0", []TriggerName{{"gethitvar", true}, {"gethitvar(hittime)", false}}},
		{"Command + 1", []TriggerName{{"command", false}}},
		{"Zork(Wibble)", []TriggerName{{"zork", true}, {"wibble", false}}},
	}
	for _, tc := range tests {
		got, err := TriggerNames(tc.expr)
		if err != nil {
			t.Errorf("TriggerNames(%q) error: %v", tc.expr, err)
			continue
		}
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("TriggerNames(%q) = %v, want %v", tc.expr, got, tc.want)
		}
	}
}

func TestTriggerNames_SyntaxError_ReturnsParseError(t *testing.T) {
	got, err := TriggerNames("Time = (")
	if err == nil {
		t.Fatalf("expected error, got names %v", got)
	}
	if got != nil {
		t.Errorf("names = %v, want nil", got)
	}
}

func TestIsKnownTrigger(t *testing.T) {
	tests := []struct {
		name string
		call bool
		want bool
	}{
		{"time", false, true},
		{"stateno", false, true},
		{"command", false, true},
		{"var", true, true},
		{"ifelse", true, true},
		{"gethitvar", true, true},
		{"var", false, false},
		{"time", true, false},
		{"inventedtrigger", false, false},
		{"inventedtrigger", true, false},
	}
	for _, tc := range tests {
		if got := IsKnownTrigger(tc.name, tc.call); got != tc.want {
			t.Errorf("IsKnownTrigger(%q, %v) = %v, want %v", tc.name, tc.call, got, tc.want)
		}
	}
}

func TestTriggerNames_GetHitVar_YieldsOneEntryPerVariable(t *testing.T) {
	got, err := TriggerNames("GetHitVar(yvel) > 0 && GetHitVar(HitTime) > 0")
	if err != nil {
		t.Fatalf("TriggerNames error: %v", err)
	}
	want := []TriggerName{{"gethitvar", true}, {"gethitvar(yvel)", false}, {"gethitvar(hittime)", false}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TriggerNames = %v, want %v", got, want)
	}
}

func TestIsKnownTrigger_GetHitVarVariable_FollowsEvalGetHitVar(t *testing.T) {
	if !IsKnownTrigger("gethitvar(hittime)", false) {
		t.Error("gethitvar(hittime) reported unknown, evalGetHitVar handles it")
	}
	if !IsKnownTrigger("gethitvar(yvel)", false) {
		t.Error("gethitvar(yvel) reported unknown, evalGetHitVar handles it")
	}
	if IsKnownTrigger("gethitvar(nosuchvar)", false) {
		t.Error("gethitvar(nosuchvar) reported known, evalGetHitVar rejects it")
	}
}

// evalCases checks each expression's value and kind against ctx.
func evalCases(t *testing.T, ctx Context, cases []struct {
	expr string
	want float64
	kind Kind
}) {
	t.Helper()
	for _, c := range cases {
		e, err := Parse(c.expr)
		if err != nil {
			t.Errorf("Parse(%q): %v", c.expr, err)
			continue
		}
		got, err := e.Eval(ctx)
		if err != nil {
			t.Errorf("Eval(%q): %v", c.expr, err)
			continue
		}
		if got.Number != c.want || got.Kind != c.kind {
			t.Errorf("%s = %v (kind %v), want %v (kind %v)", c.expr, got.Number, got.Kind, c.want, c.kind)
		}
	}
}

// Ikemen GO compiler.go:5855-5943 expRange and bytecode.go:1863 rangeCheck
// (commit 54d4148a): after = or !=, "[" or "(" opens an interval whose bounds
// are full expressions; "[" "]" include the bound, "(" ")" exclude it; the
// comparison is in floats when one of the three values is a float, else in
// ints; != negates.
func TestEval_Interval_AfterEqualOrNotEqual_ChecksTheBoundsLikeIkemen(t *testing.T) {
	ctx := Context{Anim: 5055}
	evalCases(t, ctx, []struct {
		expr string
		want float64
		kind Kind
	}{
		{"anim = [5051, 5059]", 1, KindBool},
		{"anim = [5055, 5055]", 1, KindBool},
		{"anim = (5055, 5059]", 0, KindBool},
		{"anim = [5051, 5055)", 0, KindBool},
		{"anim = (5054, 5056)", 1, KindBool},
		{"anim != [5051, 5059]", 0, KindBool},
		{"anim != [5000, 5050]", 1, KindBool},
		{"5.5 = (5, 6)", 1, KindBool},
		{"2 = [1.5, 2.5]", 1, KindBool},
		{"6 = [1.5, 2.5]", 0, KindBool},
		{"anim = [5050 + 1, 5060 - 1]", 1, KindBool},
		{"1 + 2 = [3, 3]", 1, KindBool},
		{"anim = [5051, 5059] || anim = [5061, 5069]", 1, KindBool},
		{"(anim = [5081, 5089]) || (anim = [5111, 5119])", 0, KindBool},
		// "(" with no comma is a parenthesized operand (expRange's fallback).
		{"3 = (3)", 1, KindBool},
		{"3 = (1 + 2) && 1", 1, KindBool},
		{"IfElse(anim = (5055), 4, 5)", 4, KindInt},
	})
}

func TestEval_Interval_OnARedirectionAndAnAnimElemComparison(t *testing.T) {
	ctx := duel(t, 0, match.FacingRight, 10, match.FacingLeft)
	ctx.P2.Anim = 5120
	evalCases(t, ctx, []struct {
		expr string
		want float64
		kind Kind
	}{
		{"p2, anim = [5110, 5120]", 1, KindBool},
		{"p2, anim != [5110, 5120)", 1, KindBool},
	})
	// compiler.go:1057-1121 evaluateComparison + intRange: after AnimElem's
	// comma the interval follows "=", "!=" or no operator. Action 200's
	// element 3 starts at tick 3 (see animCtx).
	for _, c := range []struct {
		expr string
		at   int
		want bool
	}{
		{"AnimElem = 3, = [0, 2]", 5, true},
		{"AnimElem = 3, = [0, 2]", 6, false},
		{"AnimElem = 3, [1, 2)", 5, false},
		{"AnimElem = 3, (1, 2]", 5, true},
		{"AnimElem = 3, != [0, 2]", 6, true},
		{"AnimElem = 3, [-1, 0]", 3, true},
	} {
		if got := mustEval(t, c.expr, animCtx(t, 200, c.at)).Bool(); got != c.want {
			t.Errorf("%s at tick %d = %v, want %v", c.expr, c.at, got, c.want)
		}
	}
}

func TestParse_MalformedInterval_IsAnErrorNamingTheExpression(t *testing.T) {
	for _, c := range []struct{ expr, want string }{
		{"anim = [5051 5059]", "','"},
		{"anim = [5051]", "','"},
		{"anim = [5051, 5059", "']' or ')'"},
		{"anim = (5051, 5059", "']' or ')'"},
		{"anim = [5051, ]", "unexpected"},
		{"AnimElem = 3, = [1, x]", "integer"},
		{"AnimElem = 3, = [1 2]", "','"},
	} {
		_, err := Parse(c.expr)
		if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), c.expr) {
			t.Errorf("Parse(%q) error = %v, want one containing %q and the expression", c.expr, err, c.want)
		}
	}
}

// Ikemen GO's precedence ladder, lowest first (compiler.go:6030-6127 and
// isOperator 645-666): || ^^ && | ^ & (= !=) (< > <= >=) (+ -) (* / %) ** and
// the unary operators of expValue (! - ~, compiler.go:1765-1823), which bind
// to the value right after them.
func TestEval_OperatorPrecedence_FollowsIkemenLadder(t *testing.T) {
	evalCases(t, Context{}, []struct {
		expr string
		want float64
		kind Kind
	}{
		// = is looser than <: 2 = (1 < 3) is 2 = 1.
		{"2 = 1 < 3", 0, KindBool},
		{"1 | 2 = 2", 1, KindInt},
		{"6 & 3 = 2", 0, KindInt},
		{"1 | 2 ^ 3 & 1", 3, KindInt},
		{"0 ^^ 1 && 0", 0, KindBool},
		{"1 || 1 ^^ 1", 1, KindBool},
		{"2 * 3 ** 2", 18, KindInt},
		{"!0 = 1", 1, KindBool},
		{"!2 + 1", 1, KindInt},
		{"-2 ** 2", 4, KindInt},
		{"-(2 + 3)", -5, KindInt},
		{"--2", 2, KindInt},
		{"~5 + 1", -5, KindInt},
	})
}

// bytecode.go:1709-1757 pow: float when an operand is a float or the exponent
// is negative, else an int32 power (wrapping); expPow (compiler.go:5739)
// loops, so ** is left-associative.
func TestEval_Power_IntFloatAndLeftAssociativity(t *testing.T) {
	evalCases(t, Context{}, []struct {
		expr string
		want float64
		kind Kind
	}{
		{"2 ** 10", 1024, KindInt},
		{"2 ** 3 ** 2", 64, KindInt},
		{"2 ** -1", 0.5, KindFloat},
		{"2.0 ** 2", 4, KindFloat},
		{"3 ** 0", 1, KindInt},
		{"2 ** 31", -2147483648, KindInt},
	})
}

// bytecode.go:1759-1789: int / int truncates, a float operand divides in
// floats; % works on the operands truncated to ints, whatever their kind.
func TestEval_DivisionAndModulo_FollowIkemenIntRules(t *testing.T) {
	evalCases(t, Context{}, []struct {
		expr string
		want float64
		kind Kind
	}{
		{"7 / 2", 3, KindInt},
		{"-7 / 2", -3, KindInt},
		{"7.0 / 2", 3.5, KindFloat},
		{"7.5 % 2", 1, KindInt},
		{"-7 % 3", -1, KindInt},
	})
}

// bytecode.go:1960-2000 bwand/bwxor/bwor/blxor, 1684 bwnot: bitwise
// operators work on int32, ^^ is a logical exclusive or.
func TestEval_BitwiseAndLogicalXor(t *testing.T) {
	evalCases(t, Context{}, []struct {
		expr string
		want float64
		kind Kind
	}{
		{"6 & 3", 2, KindInt},
		{"6 | 3", 7, KindInt},
		{"6 ^ 3", 5, KindInt},
		{"~5", -6, KindInt},
		{"~0", -1, KindInt},
		{"7.9 & 3", 3, KindInt},
		{"1 ^^ 1", 0, KindBool},
		{"2 ^^ 0", 1, KindBool},
		{"0 ^^ 0", 0, KindBool},
	})
}

// compiler.go:1888-1940: Cond evaluates its condition, then only the branch
// it picks (jz/jmp), unlike IfElse, which evaluates all three
// (compiler.go:1852-1886).
func TestEval_Cond_EvaluatesOnlyTheChosenBranch(t *testing.T) {
	evalCases(t, Context{Ctrl: true}, []struct {
		expr string
		want float64
		kind Kind
	}{
		{"cond(1, 5, var(99))", 5, KindInt},
		{"Cond(0, var(99), 7.5)", 7.5, KindFloat},
		{"cond(ctrl, 1, 2) + 1", 2, KindInt},
	})
	if _, err := Evaluate("ifelse(1, 5, var(99))", Context{}); err == nil {
		t.Error("ifelse(1, 5, var(99)) succeeded, want the out-of-range error: IfElse evaluates every branch")
	}
	ctx := Context{}
	v, err := EvaluateMut("cond(0, var(1) := 5, var(2) := 7)", &ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v.Number != 7 || ctx.Vars[1] != 0 || ctx.Vars[2] != 7 {
		t.Errorf("cond(0, var(1) := 5, var(2) := 7) = %v with var(1)=%d var(2)=%d, want 7, 0, 7", v.Number, ctx.Vars[1], ctx.Vars[2])
	}
}

func TestEval_Cond_WrongArgumentCount_IsAnError(t *testing.T) {
	for _, expr := range []string{"cond(1, 2)", "cond(1, 2, 3, 4)"} {
		_, err := Evaluate(expr, Context{})
		if err == nil || !strings.Contains(err.Error(), "3 arguments") {
			t.Errorf("Evaluate(%q) error = %v, want one about 3 arguments", expr, err)
		}
	}
	if !IsKnownTrigger("cond", true) {
		t.Error("IsKnownTrigger(cond, call) = false, want true")
	}
}

func TestEvaluate_IntervalAndCond_AllocateNothingOnTheCachedPath(t *testing.T) {
	ctx := Context{Anim: 5055, Ctrl: true}
	for _, expr := range []string{"Anim = [5051, 5059]", "Anim != (5051, 5059]", "cond(Ctrl, 1, var(99))"} {
		if _, err := Evaluate(expr, ctx); err != nil {
			t.Fatalf("Evaluate(%q): %v", expr, err)
		}
		allocs := testing.AllocsPerRun(100, func() { _, _ = Evaluate(expr, ctx) })
		if allocs != 0 {
			t.Errorf("Evaluate(%q) allocated %v times per call, want 0", expr, allocs)
		}
	}
}
