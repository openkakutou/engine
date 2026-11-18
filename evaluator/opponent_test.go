package evaluator

import (
	"errors"
	"strings"
	"testing"

	"github.com/openkakutou/engine/match"
)

// Expected values are worked out by hand from Ikemen GO's char.go (commit
// 54d4148a): rdDistX/distX 9200-9330, bodyDistX 9244-9276, p2BodyDistX/Y
// 9354-9374, and the engine's own convention that Position.Y is up where the
// content's Y is down (Pos Y = -Position.Y).

// sizeConsts is a [Size] with distinct ground and air widths so that a test
// can tell which one a body distance used.
func sizeConsts(t *testing.T) *Constants {
	t.Helper()
	c, err := ParseConstants(strings.NewReader("[Size]\nground.front = 20\nground.back = 10\nair.front = 7\nair.back = 5\n"))
	if err != nil {
		t.Fatalf("ParseConstants: %v", err)
	}
	return c
}

// duel is a fighter at selfX facing selfFacing against an opponent at oppX
// facing oppFacing, both standing, with sizeConsts for both.
func duel(t *testing.T, selfX float64, selfFacing match.Facing, oppX float64, oppFacing match.Facing) Context {
	t.Helper()
	c := sizeConsts(t)
	return Context{
		FighterState: match.FighterState{Position: match.Position{X: selfX}, Facing: selfFacing},
		StateType:    'S',
		Consts:       c,
		P2: Opponent{
			Valid:        true,
			FighterState: match.FighterState{Position: match.Position{X: oppX}, Facing: oppFacing},
			StateType:    'S',
			Consts:       c,
		},
	}
}

func TestEval_P2Dist_IsTheOpponentOffsetInTheFightersFacing(t *testing.T) {
	cases := []struct {
		name string
		ctx  Context
		expr string
		want float64
	}{
		{"X, facing right, opponent ahead", duel(t, -70, match.FacingRight, 50, match.FacingLeft), "P2Dist X", 120},
		{"X, facing left, opponent ahead", duel(t, 100, match.FacingLeft, 40, match.FacingRight), "P2Dist X", 60},
		{"X, facing right, opponent behind", duel(t, 10, match.FacingRight, -5, match.FacingRight), "P2Dist X", -15},
		{"X below Ikemen's 0.0001 snap reads 0", duel(t, 0, match.FacingRight, 0.00005, match.FacingLeft), "P2Dist X", 0},
	}
	for _, tc := range cases {
		if got := evalFloat(t, tc.expr, tc.ctx); got != tc.want {
			t.Errorf("%s: %s = %v, want %v", tc.name, tc.expr, got, tc.want)
		}
	}
}

func TestEval_P2DistY_FollowsTheContentsYAxisPointingDown(t *testing.T) {
	ctx := duel(t, 0, match.FacingRight, 10, match.FacingLeft)
	ctx.P2.Position.Y = 30 // the opponent is 30 above the ground: content Y -30
	if got := evalFloat(t, "P2Dist Y", ctx); got != -30 {
		t.Errorf("P2Dist Y = %v, want -30", got)
	}
	ctx.Position.Y = 50 // both airborne: self at content Y -50, opponent at -30
	if got := evalFloat(t, "P2Dist Y", ctx); got != 20 {
		t.Errorf("P2Dist Y with both in the air = %v, want 20 (-30 - -50)", got)
	}
}

func TestEval_P2BodyDistX_IsTheGapBetweenSizeBoxesLikeIkemenGO(t *testing.T) {
	cases := []struct {
		name string
		ctx  Context
		want float64
	}{
		// dist 60, my front edge at -50, their front edge (they face me) at -26 +... see below.
		{"facing each other, gap between the two fronts", duel(t, -70, match.FacingRight, -10, match.FacingLeft), 60 - 20 - 20},
		// Same facing, opponent ahead: their back edge (100-10) against my front (20): 70.
		{"opponent ahead, same facing, uses its back width", duel(t, 0, match.FacingRight, 100, match.FacingRight), 70},
		{"mirrored: facing left, opponent ahead, same facing", duel(t, 0, match.FacingLeft, -100, match.FacingLeft), 70},
		// Opponent behind and facing the same way: Ikemen GO's front-width branch.
		{"opponent behind, same facing", duel(t, -70, match.FacingRight, -100, match.FacingRight), -30 - 20 + 20},
	}
	for _, tc := range cases {
		if got := evalFloat(t, "P2BodyDist X", tc.ctx); got != tc.want {
			t.Errorf("%s: P2BodyDist X = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestEval_P2BodyDistX_AirborneOpponentUsesItsAirWidths(t *testing.T) {
	ctx := duel(t, 0, match.FacingRight, 100, match.FacingRight)
	ctx.P2.StateType = 'A'
	// Their back edge is air.back = 5 wide: 100 - 20 (my front) - 5.
	if got := evalFloat(t, "P2BodyDist X", ctx); got != 75 {
		t.Errorf("P2BodyDist X against an airborne opponent = %v, want 75", got)
	}
	// And mine: an airborne fighter's front is air.front = 7.
	ctx.StateType = 'A'
	if got := evalFloat(t, "P2BodyDist X", ctx); got != 100-7-5 {
		t.Errorf("P2BodyDist X with both airborne = %v, want 88", got)
	}
}

func TestEval_P2BodyDistY_IsP2DistYForAMugenCharacter(t *testing.T) {
	// Ikemen GO's p2BodyDistY returns rdDistY unless the character is an
	// Ikemen GO one (char.go:9366-9372); kfm declares mugenversion 1.0.
	ctx := duel(t, 0, match.FacingRight, 10, match.FacingLeft)
	ctx.P2.Position.Y = 30
	if got := evalFloat(t, "P2BodyDist Y", ctx); got != -30 {
		t.Errorf("P2BodyDist Y = %v, want -30", got)
	}
}

func TestEval_P2Triggers_ReadTheOpponentSnapshot(t *testing.T) {
	ctx := duel(t, 0, match.FacingRight, 10, match.FacingLeft)
	ctx.P2.StateNo = 5000
	ctx.P2.Health = 640
	ctx.P2.StateType = 'C'
	ctx.P2.MoveType = 'H'
	for expr, want := range map[string]float64{
		"P2StateNo = 5000":                   1,
		"P2StateNo":                          5000,
		"P2Life":                             640,
		"P2StateType = C":                    1,
		"P2StateType != S":                   1,
		"P2StateType = A":                    0,
		"P2MoveType = H":                     1,
		"P2MoveType = I":                     0,
		"P2MoveType != A":                    1,
		"NumEnemy":                           1,
		"P2Life > 600":                       1,
		"P2StateNo = 5000 && P2MoveType = H": 1,
	} {
		if got := evalFloat(t, expr, ctx); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
}

func TestEval_P2Triggers_WithoutAnOpponent_ReadZeroLikeIkemenUndefined(t *testing.T) {
	// No opponent: Ikemen GO's p2() is nil and its triggers undefined
	// (char.go:9318-9321), which reads 0 in a comparison.
	var ctx Context
	for _, expr := range []string{"P2Dist X", "P2Dist Y", "P2BodyDist X", "P2BodyDist Y", "P2StateNo", "P2Life", "NumEnemy", "p2,life", "p2,time", "NumTarget"} {
		if got := evalFloat(t, expr, ctx); got != 0 {
			t.Errorf("%s with no opponent = %v, want 0", expr, got)
		}
	}
}

func TestEval_Redirections_ReadTheOpponent(t *testing.T) {
	ctx := duel(t, 0, match.FacingRight, 10, match.FacingLeft)
	ctx.P2.Time = 7
	ctx.P2.Anim = 5120
	ctx.P2.Health = 300
	ctx.P2.Velocity = match.Velocity{X: 2, Y: 4} // up 4: content Vel Y -4
	ctx.P2.Facing = match.FacingLeft
	ctx.P2.GetHit = GetHit{Active: true, HitTime: 9}
	ctx.P2.Ctrl = true
	for expr, want := range map[string]float64{
		"p2,time":                     7,
		"p2, time > 1":                1,
		"P2,Anim":                     5120,
		"p2,anim = 5120":              1,
		"enemy,life":                  300,
		"enemy(0),life":               300,
		"enemyNear,life":              300,
		"enemyNear(0), life":          300,
		"p2,getHitVar(hittime)":       9,
		"p2,ctrl":                     1,
		"!p2,ctrl":                    0,
		"p2,facing":                   -1,
		"p2,vel y":                    -4,
		"p2,vel y*.5":                 -2,
		"p2,vel x":                    -2, // forward for a left-facing opponent is world -X
		"p2,alive":                    1,
		"p2,statetype = S":            1,
		"p2,movetype != H":            1,
		"p2,const(size.ground.front)": 20,
		"ifelse(p2,life < 400, 5, 6)": 5,
		"enemy(1),life":               0, // there is no second enemy: undefined, 0
		"enemyNear(1),life":           0,
	} {
		if got := evalFloat(t, expr, ctx); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
}

func TestParse_RedirectionToAHelperFamily_IsNotSupportedYet(t *testing.T) {
	for _, expr := range []string{"root,life", "parent,life", "helper(3),life", "playerid(2),life", "partner,life", "stateowner,life", "player(2),life"} {
		_, err := Parse(expr)
		if err == nil || !strings.Contains(err.Error(), "not supported yet") {
			t.Errorf("Parse(%q) error = %v, want one saying the redirection is not supported yet", expr, err)
		}
	}
}

func TestEval_Redirection_OfATriggerTheSnapshotLacks_IsAnUnknownTriggerNotAWrongValue(t *testing.T) {
	ctx := duel(t, 0, match.FacingRight, 10, match.FacingLeft)
	for _, expr := range []string{"p2,var(0)", "p2,numProj", "p2,command = \"a\"", "p2,random"} {
		e, err := Parse(expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", expr, err)
		}
		_, err = e.Eval(ctx)
		var unknown *UnknownTriggerError
		if !errors.As(err, &unknown) {
			t.Errorf("Eval(%q) error = %v, want an UnknownTriggerError", expr, err)
		}
	}
}

func TestEval_NumTarget_CountsTheOpponentWhileItIsHitByTheFighter(t *testing.T) {
	ctx := duel(t, 0, match.FacingRight, 10, match.FacingLeft)
	if got := evalFloat(t, "NumTarget", ctx); got != 0 {
		t.Errorf("NumTarget before any hit = %v, want 0", got)
	}
	ctx.HasTarget = true
	ctx.P2.MoveType = 'H'
	ctx.P2.GetHit = GetHit{Active: true, HitID: 4}
	for expr, want := range map[string]float64{"NumTarget": 1, "NumTarget(-1)": 1, "NumTarget(4)": 1, "NumTarget(5)": 0, "target,getHitVar(hittime)": 0, "p2,getHitVar(hitshaketime)": 0} {
		if got := evalFloat(t, expr, ctx); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
	ctx.P2.MoveType = 'I' // recovered: Ikemen GO drops a target that left the hit state (char.go:10387-10404)
	if got := evalFloat(t, "NumTarget", ctx); got != 0 {
		t.Errorf("NumTarget once the opponent is not in a hit state = %v, want 0", got)
	}
}

func TestEval_TargetRedirection_ReadsTheHitOpponent(t *testing.T) {
	ctx := duel(t, 0, match.FacingRight, 10, match.FacingLeft)
	ctx.HasTarget = true
	ctx.P2.MoveType = 'H'
	ctx.P2.GetHit = GetHit{Active: true, HitTime: 6}
	if got := evalFloat(t, "target,getHitVar(hittime)", ctx); got != 6 {
		t.Errorf("target,getHitVar(hittime) = %v, want 6", got)
	}
	ctx.HasTarget = false
	if got := evalFloat(t, "target,getHitVar(hittime)", ctx); got != 0 {
		t.Errorf("target,getHitVar(hittime) without a target = %v, want 0 (undefined)", got)
	}
}

func TestEval_P2DistWithAZAxis_IsAnError(t *testing.T) {
	e, err := Parse("P2Dist Z")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Eval(duel(t, 0, match.FacingRight, 1, match.FacingLeft)); err == nil || !strings.Contains(err.Error(), "no depth") {
		t.Errorf("P2Dist Z error = %v, want the no-depth error", err)
	}
}

func TestTriggerNames_Redirection_NamesTheRedirectedTrigger(t *testing.T) {
	names, err := TriggerNames("p2,numProj > 0 && p2BodyDist x < 3")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, n := range names {
		got[n.Name] = true
	}
	if !got["numproj"] || !got["p2bodydist"] {
		t.Errorf("TriggerNames = %v, want it to hold numproj and p2bodydist", names)
	}
	if IsKnownTrigger("numproj", false) {
		t.Error("numproj is reported known")
	}
	for _, name := range []string{"p2dist", "p2bodydist", "p2stateno", "p2movetype", "p2statetype", "p2life", "numenemy", "numtarget", "movecontact", "movehit", "moveguarded", "movereversed", "hitfall", "canrecover", "hitdefattr"} {
		if !IsKnownTrigger(name, name == "numtarget") && !IsKnownTrigger(name, false) {
			t.Errorf("IsKnownTrigger(%q) = false, want true", name)
		}
	}
}

func TestEval_OpponentAndContactTriggersWarmPath_AllocateNothing(t *testing.T) {
	ctx := duel(t, -70, match.FacingRight, -10, match.FacingLeft)
	ctx.MoveContactTime = 2
	exprs := []string{"P2Dist X", "P2BodyDist X", "p2,life", "p2,vel y*.5", "MoveContact", "NumTarget", "p2,hitDefAttr = SC,AT", "P2StateType = S", "GetHitVar(yvel)"}
	parsed := make([]*Expression, len(exprs))
	for i, expr := range exprs {
		e, err := Parse(expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", expr, err)
		}
		parsed[i] = e
	}
	allocs := testing.AllocsPerRun(100, func() {
		for _, e := range parsed {
			if _, err := e.EvalMut(&ctx); err != nil {
				t.Fatal(err)
			}
		}
	})
	if allocs != 0 {
		t.Errorf("evaluating the opponent and contact triggers allocated %v times, want 0", allocs)
	}
}
