package evaluator

import (
	"errors"
	"math"
	"testing"

	"github.com/openkakutou/engine/match"
)

func fighterCtx() Context {
	return Context{
		FighterState: match.FighterState{
			Health:   750,
			Power:    1200,
			Facing:   match.FacingLeft,
			Position: match.Position{X: -60, Y: 25},
			Velocity: match.Velocity{X: 3.5, Y: 4},
		},
		StateType:   'A',
		MoveType:    'H',
		PrevStateNo: 120,
		Round:       2,
	}
}

func evalFloat(t *testing.T, expr string, ctx Context) float64 {
	t.Helper()
	return mustEval(t, expr, ctx).Float()
}

func TestEval_OwnFighterTriggers_ReadLiveState(t *testing.T) {
	ctx := fighterCtx()
	cases := []struct {
		expr string
		want float64
	}{
		{"Life", 750},
		{"Power", 1200},
		{"Alive", 1},
		{"Facing", -1},
		{"PrevStateNo", 120},
		{"RoundNo", 2},
		{"RoundState", 2},
		{"IsHelper", 0},
		{"IsHelper(1000)", 0},
		{"Pos X", -60},
		{"Vel X", -3.5}, // facing left: world +3.5 is backward
		// MUGEN's Y axis points down: engine Y up 25 is Pos Y -25.
		{"Pos Y", -25},
		{"Vel Y", -4},
	}
	for _, c := range cases {
		if got := evalFloat(t, c.expr, ctx); got != c.want {
			t.Errorf("%s = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestEval_Facing_RightIsPlusOne(t *testing.T) {
	if got := evalFloat(t, "Facing", Context{}); got != 1 {
		t.Errorf("Facing for a right-facing fighter = %v, want 1", got)
	}
}

func TestEval_Alive_IsFalseAtZeroHealth(t *testing.T) {
	if got := mustEval(t, "Alive", Context{FighterState: match.FighterState{Health: 0}}); got.Bool() {
		t.Error("Alive at 0 health = true, want false")
	}
}

func TestEval_StateTypeAndMoveType_CompareToLettersCaseInsensitively(t *testing.T) {
	ctx := fighterCtx() // A, H
	cases := []struct {
		expr string
		want bool
	}{
		{"StateType = A", true},
		{"StateType = a", true},
		{"StateType = S", false},
		{"StateType != S", true},
		{"StateType != A", false},
		{"MoveType = H", true},
		{"MoveType != H", false},
		{"MoveType = I", false},
		{"Statetype = A && MoveType = H", true},
	}
	for _, c := range cases {
		if got := mustEval(t, c.expr, ctx).Bool(); got != c.want {
			t.Errorf("%s = %v, want %v", c.expr, got, c.want)
		}
	}
}

func TestEval_StateTypeWithAnUnknownLetter_IsAnErrorNamingIt(t *testing.T) {
	e, err := Parse("StateType = Q")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Eval(fighterCtx()); err == nil {
		t.Error("StateType = Q evaluated without error, want an error naming the letter")
	}
}

func TestEval_MathFunctions_FollowIkemenGOResultTypes(t *testing.T) {
	cases := []struct {
		expr string
		want float64
		kind Kind
	}{
		{"Abs(-3)", 3, KindInt},
		{"Abs(-2.5)", 2.5, KindFloat},
		{"Floor(2.7)", 2, KindInt},
		{"Floor(-2.2)", -3, KindInt},
		{"Ceil(2.1)", 3, KindInt},
		{"Ceil(-2.7)", -2, KindInt},
		{"Floor(5)", 5, KindInt},
		{"Sin(0)", 0, KindFloat},
		{"Cos(0)", 1, KindFloat},
		{"Tan(0)", 0, KindFloat},
		{"ASin(1)", float64(float32(math.Pi / 2)), KindFloat},
		{"ACos(1)", 0, KindFloat},
		{"ATan(1)", float64(float32(math.Pi / 4)), KindFloat},
		{"Exp(0)", 1, KindFloat},
		{"Ln(1)", 0, KindFloat},
		// Log(a, b) is the logarithm of b in base a.
		{"Log(2, 8)", float64(float32(math.Log(8) / math.Log(2))), KindFloat},
	}
	for _, c := range cases {
		got := mustEval(t, c.expr, Context{})
		if got.Number != c.want || got.Kind != c.kind {
			t.Errorf("%s = %v (kind %v), want %v (kind %v)", c.expr, got.Number, got.Kind, c.want, c.kind)
		}
	}
}

func TestEval_InvalidLogarithm_IsAnError(t *testing.T) {
	for _, expr := range []string{"Ln(0)", "Ln(-1)", "Log(2, 0)", "Log(0, 2)"} {
		e, err := Parse(expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", expr, err)
		}
		if _, err := e.Eval(Context{}); err == nil {
			t.Errorf("%s evaluated without error, want an invalid-logarithm error", expr)
		}
	}
}

func TestEval_MathFunction_WithWrongArgumentCount_IsAnError(t *testing.T) {
	for _, expr := range []string{"Abs()", "Abs(1, 2)", "Log(2)"} {
		e, err := Parse(expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", expr, err)
		}
		if _, err := e.Eval(Context{}); err == nil {
			t.Errorf("%s evaluated without error, want an argument-count error", expr)
		}
	}
}

func TestEvalMut_Random_FollowsIkemenGOsGeneratorFromTheSeed(t *testing.T) {
	// Expected values computed independently from Ikemen GO's common.go
	// (Park-Miller minimal standard, Rand(0, 999)), seed 1.
	want := []int{0, 131, 755, 458, 532}
	ctx := Context{RandSeed: 1}
	for i, w := range want {
		v, err := EvaluateMut("Random", &ctx)
		if err != nil {
			t.Fatalf("draw %d: %v", i, err)
		}
		if v.Int() != w {
			t.Errorf("draw %d = %d, want %d", i, v.Int(), w)
		}
	}
}

func TestEvalMut_Random_TwoRunsFromTheSameSeedAreIdentical(t *testing.T) {
	a, b := Context{RandSeed: 12345}, Context{RandSeed: 12345}
	for i := 0; i < 50; i++ {
		va, _ := EvaluateMut("Random", &a)
		vb, _ := EvaluateMut("Random", &b)
		if va != vb {
			t.Fatalf("draw %d differs: %v vs %v", i, va, vb)
		}
	}
	if a.RandSeed != b.RandSeed {
		t.Errorf("final seeds differ: %d vs %d", a.RandSeed, b.RandSeed)
	}
}

func TestEvalMut_Random_StaysInZeroTo999(t *testing.T) {
	ctx := Context{RandSeed: 99}
	for i := 0; i < 2000; i++ {
		v, _ := EvaluateMut("Random", &ctx)
		if v.Int() < 0 || v.Int() > 999 {
			t.Fatalf("Random = %d, outside 0..999", v.Int())
		}
	}
}

func TestEvalMut_CNSAndOr_EvaluateBothOperandsSoRandomStillDraws(t *testing.T) {
	// Ikemen GO compiles && and || inside a CNS controller as OC_bland and
	// OC_blor: both sides run, so the seed advances even when the left
	// side already decides the result.
	for _, expr := range []string{"0 && Random < 500", "1 || Random < 500"} {
		ctx := Context{RandSeed: 1}
		if _, err := EvaluateMut(expr, &ctx); err != nil {
			t.Fatal(err)
		}
		if ctx.RandSeed != 16807 {
			t.Errorf("%s: seed = %d, want 16807 (one draw)", expr, ctx.RandSeed)
		}
	}
}

func TestEvalMutShortCircuit_ZSSAndOr_SkipTheDecidedRightOperand(t *testing.T) {
	for _, expr := range []string{"0 && Random < 500", "1 || Random < 500"} {
		ctx := Context{RandSeed: 1}
		if _, err := EvaluateMutShortCircuit(expr, &ctx); err != nil {
			t.Fatal(err)
		}
		if ctx.RandSeed != 1 {
			t.Errorf("%s: seed = %d, want 1 (right operand skipped)", expr, ctx.RandSeed)
		}
	}
	ctx := Context{RandSeed: 1}
	if _, err := EvaluateMutShortCircuit("1 && Random < 500", &ctx); err != nil {
		t.Fatal(err)
	}
	if ctx.RandSeed != 16807 {
		t.Errorf("seed = %d, want 16807 when the right operand of && is needed", ctx.RandSeed)
	}
}

func TestEvalMut_IfElse_EvaluatesTheBranchNotTaken(t *testing.T) {
	ctx := Context{RandSeed: 1}
	v, err := EvaluateMut("IfElse(0, Random, 7)", &ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v.Int() != 7 || ctx.RandSeed != 16807 {
		t.Errorf("value=%d seed=%d, want 7 and 16807 (Random in the untaken branch still draws)", v.Int(), ctx.RandSeed)
	}
}

func TestEval_VelX_IsRelativeToFacing(t *testing.T) {
	left := Context{FighterState: match.FighterState{Facing: match.FacingLeft, Velocity: match.Velocity{X: -3}}}
	if got := evalFloat(t, "Vel X", left); got != 3 {
		t.Errorf("Vel X of a left-facing fighter moving left = %v, want 3 (forward)", got)
	}
	if got := evalFloat(t, "Pos X", Context{FighterState: match.FighterState{Facing: match.FacingLeft, Position: match.Position{X: -40}}}); got != -40 {
		t.Errorf("Pos X of a left-facing fighter = %v, want -40 (not facing-relative)", got)
	}
}

func TestEval_Random_WithoutMutation_DoesNotAdvanceTheCallersContext(t *testing.T) {
	ctx := Context{RandSeed: 1}
	mustEval(t, "Random", ctx)
	if ctx.RandSeed != 1 {
		t.Errorf("Eval advanced the caller's seed to %d; only EvalMut may", ctx.RandSeed)
	}
}

func TestIsKnownTrigger_ImplementedNames_AreKnownAndUnknownStaysUnknown(t *testing.T) {
	for _, name := range []string{"life", "power", "alive", "facing", "prevstateno", "roundno", "roundstate", "ishelper", "random", "pos", "vel", "statetype", "movetype"} {
		if !IsKnownTrigger(name, false) {
			t.Errorf("IsKnownTrigger(%q, false) = false, want true", name)
		}
	}
	for _, name := range []string{"abs", "floor", "ceil", "sin", "cos", "tan", "asin", "acos", "atan", "exp", "ln", "log", "ishelper"} {
		if !IsKnownTrigger(name, true) {
			t.Errorf("IsKnownTrigger(%q, true) = false, want true", name)
		}
	}
	if IsKnownTrigger("nosuchtrigger", false) || IsKnownTrigger("nosuchfunc", true) {
		t.Error("an unimplemented name was reported as known")
	}
}

func TestEval_UnknownTrigger_StillNamesItInTheError(t *testing.T) {
	e, err := Parse("NoSuchTrigger = 1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = e.Eval(Context{})
	var ute *UnknownTriggerError
	if !errors.As(err, &ute) || ute.Name != "nosuchtrigger" {
		t.Errorf("err = %v, want an UnknownTriggerError naming nosuchtrigger", err)
	}
}

func TestTriggerNames_PosAndVelComponents_AreReportedAsTheirTrigger(t *testing.T) {
	names, err := TriggerNames("Pos Y < 0 && Vel X > 1")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, n := range names {
		got[n.Name] = true
	}
	if !got["pos"] || !got["vel"] || got["y"] || got["x"] {
		t.Errorf("trigger names = %v, want pos and vel, not the axis letters", got)
	}
}
