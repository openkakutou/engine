package evaluator

import (
	"errors"
	"math"
	"testing"

	"github.com/openkakutou/engine/v2/match"
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

// bytecode.go:2510-2515 (OC_ailevel) and :3654-3659 (OC_ex_ailevelf) read
// sys.aiLevel[playerNo], which is 0 for a human player; every engine fighter
// is human-driven.
func TestEval_AILevelAndAILevelF_AreZeroForAHumanDrivenFighter(t *testing.T) {
	if got := mustEval(t, "AILevel", fighterCtx()); got.Number != 0 || got.Kind != KindInt {
		t.Errorf("AILevel = %v kind %v, want int 0", got.Number, got.Kind)
	}
	if got := mustEval(t, "AILevelF", fighterCtx()); got.Number != 0 || got.Kind != KindFloat {
		t.Errorf("AILevelF = %v kind %v, want float 0", got.Number, got.Kind)
	}
	// kfm_zss command.zss state -1 opens with `roundState = 2 && !aiLevelF`.
	if !mustEval(t, "roundState = 2 && !aiLevelF", fighterCtx()).Bool() {
		t.Error("roundState = 2 && !aiLevelF is false for a human fighter")
	}
}

// char.go:5528-5537 getAILevel: AILevel truncates the level, AILevelF keeps it.
func TestEval_AILevelFollowsTheLevelTheGameGivesACPUFighter(t *testing.T) {
	ctx := fighterCtx()
	ctx.AILevel = 4.7
	if got := mustEval(t, "AILevel", ctx); got.Number != 4 || got.Kind != KindInt {
		t.Errorf("AILevel = %v kind %v, want int 4", got.Number, got.Kind)
	}
	if got := mustEval(t, "AILevelF", ctx); math.Abs(got.Number-4.7) > 1e-6 || got.Kind != KindFloat {
		t.Errorf("AILevelF = %v kind %v, want float 4.7", got.Number, got.Kind)
	}
	if mustEval(t, "roundState = 2 && !aiLevelF", ctx).Bool() {
		t.Error("a CPU fighter must not take the human branch of kfm_zss")
	}
}

// bytecode.go:2671-2672 reads gi().palno; char.go:4888-4913 keeps it at 1 or
// above, 1 when no palette was chosen.
func TestEval_PalNo_DefaultsToOneAndFollowsTheContext(t *testing.T) {
	if got := mustEval(t, "PalNo", Context{}); got.Number != 1 {
		t.Errorf("PalNo = %v, want 1 when the game sets none", got.Number)
	}
	if got := mustEval(t, "PalNo", Context{PalNo: 4}); got.Number != 4 {
		t.Errorf("PalNo = %v, want 4", got.Number)
	}
}

// bytecode.go:2657-2660: NumExplod/NumHelper take an optional id (compiler.go
// :3415-3425, default -1) and count the player's explods/helpers, none of
// which the engine has yet (backlog items 029 and 052).
func TestEval_NumHelperAndNumExplod_AreZeroWithOrWithoutAnId(t *testing.T) {
	for _, expr := range []string{"NumHelper", "NumHelper(1000)", "NumExplod", "NumExplod(0)", "NumHelper(1+2)"} {
		if got := mustEval(t, expr, Context{}); got.Number != 0 || got.Kind != KindInt {
			t.Errorf("%s = %v kind %v, want int 0", expr, got.Number, got.Kind)
		}
	}
	if _, err := Evaluate("NumHelper(1, 2)", Context{}); err == nil {
		t.Error("NumHelper with two arguments returned no error")
	}
}

func TestIsKnownTrigger_OtherFamily_IsKnown(t *testing.T) {
	for _, name := range []string{"ailevel", "ailevelf", "palno", "numhelper", "numexplod"} {
		if !IsKnownTrigger(name, false) {
			t.Errorf("IsKnownTrigger(%q, false) = false", name)
		}
	}
	for _, name := range []string{"const", "const240p", "const480p", "const720p", "const1080p", "fvar", "sysfvar", "numhelper", "numexplod"} {
		if !IsKnownTrigger(name, true) {
			t.Errorf("IsKnownTrigger(%q, true) = false", name)
		}
	}
}

// bytecode.go:3829-3831 reads c.mapArray[name], the zero value when the
// name was never set; compiler.go:5197-5230 takes a bare name as a literal
// key, a quoted one too. Names are lower-cased (char.go:9813).
func TestEval_MapReadsTheFightersNamedValueOrZero(t *testing.T) {
	ctx := fighterCtx()
	ctx.SetMap("canCombo", intValue(3))
	ctx.SetMap("ppx", floatValue(1.5))
	if got := mustEval(t, "map(CanCombo)", ctx); got.Number != 3 || got.Kind != KindInt {
		t.Errorf("map(CanCombo) = %v kind %v, want int 3", got.Number, got.Kind)
	}
	if got := mustEval(t, `map("ppx") * 2`, ctx); got.Number != 3 {
		t.Errorf(`map("ppx") * 2 = %v, want 3`, got.Number)
	}
	if got := mustEval(t, "map(never)", ctx); got.Number != 0 {
		t.Errorf("map(never) = %v, want 0", got.Number)
	}
}

// char.go:5620-5646 and 5759-5784: the edge distances measure from the
// fighter to the screen edges sys.xmin / sys.xmax, behind and in front
// according to its facing; the body variants subtract widthEdge, 0 unless a
// Width controller set it.
func TestEval_EdgeDistancesFollowFacing(t *testing.T) {
	ctx := fighterCtx()
	ctx.ScreenLeft, ctx.ScreenRight = -100, 200
	ctx.Facing = match.FacingRight
	ctx.StateType = 'S' // standing: no MUGEN body offset
	ctx.Position.X = 20 // 120 to the left edge, 180 to the right one
	want := map[string]float64{"BackEdgeDist": 120, "FrontEdgeDist": 180, "BackEdgeBodyDist": 120, "FrontEdgeBodyDist": 180}
	for expr, w := range want {
		if got := mustEval(t, expr, ctx); got.Number != w {
			t.Errorf("facing right: %s = %v, want %v", expr, got.Number, w)
		}
	}
	ctx.Facing = match.FacingLeft
	for expr, w := range map[string]float64{"BackEdgeDist": 180, "FrontEdgeDist": 120} {
		if got := mustEval(t, expr, ctx); got.Number != w {
			t.Errorf("facing left: %s = %v, want %v", expr, got.Number, w)
		}
	}
}

// char.go:14159-14183: a fighter is in guard distance while its attacking
// opponent's HitDef reaches it: distX = (self x - opponent x) * opponent
// facing, inside (-guard.dist back, guard.dist front), strictly.
func TestEval_InGuardDistFollowsTheOpponentsHitDef(t *testing.T) {
	ctx := fighterCtx()
	ctx.Position.X = 100
	ctx.P2 = Opponent{Valid: true, MoveType: 'A', HitDefAttr: 1, GuardDist: [2]float64{60, 10}}
	ctx.P2.Position.X = 50
	ctx.P2.Facing = match.FacingRight
	if !mustEval(t, "InGuardDist", ctx).Bool() {
		t.Error("50 ahead of an attacker reaching 60: want in guard distance")
	}
	ctx.P2.Position.X = 40 // 60 ahead: not strictly inside
	if mustEval(t, "InGuardDist", ctx).Bool() {
		t.Error("exactly at the reach: want out (Ikemen GO uses <)")
	}
	ctx.P2.Position.X = 105 // 5 behind the attacker, back reach 10
	if !mustEval(t, "InGuardDist", ctx).Bool() {
		t.Error("5 behind with a back reach of 10: want in")
	}
	ctx.P2.Position.X = 50
	ctx.P2.MoveType = 'I'
	if mustEval(t, "InGuardDist", ctx).Bool() {
		t.Error("an opponent not attacking: want out")
	}
	ctx.P2.MoveType, ctx.P2.HitDefAttr = 'A', 0
	if mustEval(t, "InGuardDist", ctx).Bool() {
		t.Error("an attacking opponent without an active HitDef: want out")
	}
}

// char.go:6186-6191 counts the fighter's live projectiles; the engine has
// none yet (backlog item 024), so a fighter, or its opponent, has 0.
func TestEval_NumProjIsZeroWithoutProjectiles(t *testing.T) {
	ctx := fighterCtx()
	ctx.P2 = Opponent{Valid: true}
	if got := mustEval(t, "NumProj + p2,NumProj", ctx); got.Number != 0 || got.Kind != KindInt {
		t.Errorf("NumProj + p2,NumProj = %v kind %v, want int 0", got.Number, got.Kind)
	}
}

// bytecode.go:2615-2616 reads c.hitPauseTime, the attacker's remaining pause.
func TestEval_HitPauseTimeReadsTheRemainingPause(t *testing.T) {
	ctx := fighterCtx()
	ctx.HitPauseTime = 7
	if got := mustEval(t, "HitPauseTime", ctx); got.Number != 7 || got.Kind != KindInt {
		t.Errorf("HitPauseTime = %v kind %v, want int 7", got.Number, got.Kind)
	}
}

// system.go:2277-2325 numbers the players from HelperMax (56 by default):
// P1 is 56, P2 57, as in MUGEN (bytecode.go:2626-2627).
func TestEval_IDIsFiftySixPlusTheSide(t *testing.T) {
	ctx := fighterCtx()
	ctx.Side = match.SideP2
	ctx.P2 = Opponent{Valid: true}
	ctx.P2.Side = match.SideP1
	if got := mustEval(t, "ID * 100 + p2,ID", ctx); got.Number != 5756 {
		t.Errorf("ID * 100 + p2,ID = %v, want 5756", got.Number)
	}
}

// char.go:5933-5946: a non-target's juggle points are the attacker's maximum,
// data.airjuggle (15 by default).
// The engine does not spend juggle points yet, so a target's
// are the maximum too.
func TestEval_JugglePointsIsTheMaximumWithoutJuggleTracking(t *testing.T) {
	ctx := fighterCtx()
	ctx.P2 = Opponent{Valid: true}
	if got := mustEval(t, "JugglePoints(p2, ID)", ctx); got.Number != 15 {
		t.Errorf("JugglePoints(p2, ID) = %v, want data.airjuggle 15", got.Number)
	}
}

// char.go:5627-5639: a MUGEN character's body distance to an edge is 0.5
// less in the air and 1 less lying down; an Ikemen GO character's is not.
func TestEval_EdgeBodyDistOffsetOfMugenCharacters(t *testing.T) {
	ctx := fighterCtx()
	ctx.ScreenLeft, ctx.ScreenRight = -100, 200
	ctx.Position.X = 20
	ctx.Facing = match.FacingRight
	ctx.StateType = 'A'
	if got := mustEval(t, "BackEdgeBodyDist", ctx); got.Number != 119.5 {
		t.Errorf("MUGEN, air: %v, want 119.5", got.Number)
	}
	ctx.StateType = 'L'
	if got := mustEval(t, "FrontEdgeBodyDist", ctx); got.Number != 179 {
		t.Errorf("MUGEN, lying: %v, want 179", got.Number)
	}
	if got := mustEval(t, "FrontEdgeDist", ctx); got.Number != 180 {
		t.Errorf("the plain distance has no offset: %v, want 180", got.Number)
	}
	ctx.IkemenChar = true
	if got := mustEval(t, "FrontEdgeBodyDist", ctx); got.Number != 180 {
		t.Errorf("Ikemen GO, lying: %v, want 180", got.Number)
	}
}
