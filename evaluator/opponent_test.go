package evaluator

import (
	"errors"
	"strings"
	"testing"

	"github.com/openkakutou/engine/v2/match"
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

func TestParse_RedirectionToAnotherPlayer_IsNotSupportedYet(t *testing.T) {
	for _, expr := range []string{"stateowner,life"} {
		_, err := Parse(expr)
		if err == nil || !strings.Contains(err.Error(), "not supported yet") {
			t.Errorf("Parse(%q) error = %v, want one saying the redirection is not supported yet", expr, err)
		}
	}
}

func TestEval_Redirection_OfAnUnknownTrigger_IsAnUnknownTrigger(t *testing.T) {
	ctx := duel(t, 0, match.FacingRight, 10, match.FacingLeft)
	for _, expr := range []string{"p2,noSuchTrigger", "p2,noSuchFunction(1)"} {
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
	names, err := TriggerNames("p2,noSuchTrigger > 0 && p2BodyDist x < 3")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, n := range names {
		got[n.Name] = true
	}
	if !got["nosuchtrigger"] || !got["p2bodydist"] {
		t.Errorf("TriggerNames = %v, want it to hold nosuchtrigger and p2bodydist", names)
	}
	if IsKnownTrigger("nosuchtrigger", false) {
		t.Error("nosuchtrigger is reported known")
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

// A redirection reads the opponent's edge distances on the same screen
// (kfm_zss AI.zss reads p2,backEdgeBodyDist).
func TestEval_RedirectedEdgeDistances(t *testing.T) {
	ctx := duel(t, 0, match.FacingRight, 50, match.FacingLeft)
	ctx.ScreenLeft, ctx.ScreenRight = -100, 200
	ctx.P2.ScreenLeft, ctx.P2.ScreenRight = -100, 200
	// P2 at 50 facing left: its back is the right edge, 150 away.
	if got := mustEval(t, "p2,BackEdgeBodyDist", ctx); got.Number != 150 {
		t.Errorf("p2,BackEdgeBodyDist = %v, want 150", got.Number)
	}
	if got := mustEval(t, "p2,FrontEdgeDist", ctx); got.Number != 150 {
		t.Errorf("p2,FrontEdgeDist = %v, want 150", got.Number)
	}
}

// A helper reads its parent and its root (char.go:5340-5382), and a player
// its helpers by id (char.go:5383-5410), through the snapshots the engine
// gives it; var(n) and fvar(n) read through them.
func TestEval_HelperRedirections_ReadParentRootAndHelpers(t *testing.T) {
	ctx := Context{IsHelper: true, HelperID: 7}
	ctx.Parent = Opponent{Valid: true, IsHelper: true, HelperID: 3}
	ctx.Parent.StateNo = 1100
	ctx.Root = Opponent{Valid: true}
	ctx.Root.StateNo = 1000
	ctx.Root.Vars[5] = 9
	ctx.Root.FVars[2] = 1.5
	ctx.Root.HitPauseTime = 3
	ctx.Round = 2
	h1 := Opponent{Valid: true, IsHelper: true, HelperID: 3}
	h1.StateNo = 1100
	h2 := Opponent{Valid: true, IsHelper: true, HelperID: 7}
	h2.StateNo = 1200
	ctx.Helpers = []Opponent{h1, h2}
	ctx.P2 = Opponent{Valid: true, Helpers: []Opponent{h1}}
	for expr, want := range map[string]float64{
		"parent,stateno": 1100, "root,stateno": 1000, "root,var(5)": 9, "root,fvar(2)": 1.5,
		"parent,ishelper": 1, "root,ishelper": 0, "root,hitpausetime": 3, "root,roundno": 2,
		"helper(7),stateno": 1200, "helper,stateno": 1100, "helper(9),stateno": 0,
		"ishelper": 1, "ishelper(7)": 1, "ishelper(3)": 0, "ishelper(-1)": 1,
		"numhelper": 2, "numhelper(3)": 1, "numhelper(0)": 2, "numhelper(5)": 0,
		"enemynear,numhelper": 1, "p2,numhelper(7)": 0,
	} {
		if got := evalFloat(t, expr, ctx); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
}

// A player is no helper, and has no parent or root to redirect to: those
// read 0, as Ikemen GO's nil char.
func TestEval_APlayerHasNoParentOrRoot(t *testing.T) {
	ctx := Context{}
	for _, expr := range []string{"ishelper", "parent,stateno", "root,life", "numhelper"} {
		if got := evalFloat(t, expr, ctx); got != 0 {
			t.Errorf("%s = %v, want 0", expr, got)
		}
	}
}

// compiler.go:3396-3413, bytecode.go:1909-1923 and 3101-3114: the name
// triggers compare case-insensitively; a missing opponent or partner is
// neither equal nor unequal.
func TestEval_NameTriggers(t *testing.T) {
	ctx := Context{Name: "Kung Fu Man", AuthorName: "Elecbyte"}
	ctx.P2 = Opponent{Valid: true, Name: "Ryu", AuthorName: "Soy Sauce"}
	for expr, want := range map[string]float64{
		`name = "kung fu man"`: 1, `P1Name = "Kung Fu Man"`: 1, `name != "Ryu"`: 1,
		`p2name = "RYU"`: 1, `"ryu" = p2name`: 1, `p2name != "Ryu"`: 0,
		`authorname = "elecbyte"`: 1, `p3name = "Ryu"`: 0, `p4name != "Ryu"`: 0,
		`enemy,name = "ryu"`: 1, `enemy,authorname = "Soy Sauce"`: 1, `enemy(1),name = "Ryu"`: 0, `enemynear,name != "Ken"`: 1,
	} {
		if got := evalFloat(t, expr, ctx); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
	alone := Context{Name: "Kung Fu Man"}
	for _, expr := range []string{`p2name = "Ryu"`, `p2name != "Ryu"`} {
		if got := evalFloat(t, expr, alone); got != 0 {
			t.Errorf("without an opponent %s = %v, want 0", expr, got)
		}
	}
}

// kfm.cns state 1300: "Time = command = \"holdfwd\"" is Time compared to
// the command test (compiler.go:2197-2238).
func TestEval_CommandBindsItsOwnComparison(t *testing.T) {
	ctx := Context{ActiveCommands: map[string]bool{"holdfwd": true}}
	ctx.Time = 1
	if got := evalFloat(t, `Time = command = "holdfwd"`, ctx); got != 1 {
		t.Errorf("= %v, want 1 (Time 1 = holding forward)", got)
	}
	ctx.Time = 0
	if got := evalFloat(t, `Time = command != "holdfwd"`, ctx); got != 1 {
		t.Errorf("= %v, want 1 (Time 0 = not not-holding)", got)
	}
}

func TestEval_StateTimeIsTime(t *testing.T) {
	ctx := Context{}
	ctx.Time = 7
	if got := evalFloat(t, "statetime >= 7", ctx); got != 1 {
		t.Errorf("statetime >= 7 = %v, want 1", got)
	}
}

func TestEval_TeamSide(t *testing.T) {
	ctx := Context{}
	ctx.Side = 1
	ctx.P2 = Opponent{Valid: true}
	if got := evalFloat(t, "teamside", ctx); got != 2 {
		t.Errorf("P2 teamside = %v, want 2", got)
	}
	if got := evalFloat(t, "enemy,teamside", ctx); got != 1 {
		t.Errorf("enemy,teamside = %v, want 1", got)
	}
}

func TestEval_SurveyTriggers(t *testing.T) {
	ctx := Context{Round: 3, IsHelper: true}
	ctx.Time = 6
	ctx.Position.X, ctx.Position.Y = 10, 5
	ctx.Parent = Opponent{Valid: true}
	ctx.Parent.Position.X, ctx.Parent.Position.Y = 40, 0
	ctx.Root = Opponent{Valid: true}
	ctx.Root.Position.X = -20
	ctx.ScreenLeft = -100
	ctx.GameTime = 50
	ctx.ProjContacts = []ProjContact{{ID: 1000, HitTick: 47}}
	ctx.ProjectileIDs = []int{1000, 1000, 7}
	for expr, want := range map[string]float64{
		"roundsexisted": 2, "ishometeam": 1, "winko": 0, "lose": 0, "matchover": 0, "matchno": 1,
		"lifemax": 1000, "powermax": 3000, "gamewidth": 320, "gameheight": 240, "camerazoom": 1,
		"screenwidth": 320, "screenheight": 240,
		"teammode = single": 1, "teammode != single": 0, "teammode = simul": 0, "teammode = Turns": 0,
		"timemod = 2, 0": 1, "timemod = 4, 2": 1, "timemod = 4, 1": 0,
		"projhit1000 = 1": 0, "projhit1000 = 1, = 2": 1, "projhit1000 = 1, > 5": 0, "projhit1000 = 0, > 5": 1,
		"projhit7 = 1, >= 0": 0, "projhittime(1000)": 2, "projhittime(7)": -1, "numprojid(1000)": 2,
		"parentdist x": 30, "parentdist y": 5, "rootdist x": -30, "screenpos x": 110, "screenpos y": 215,
		"partner,life": 0,
	} {
		if got := evalFloat(t, expr, ctx); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
}

func TestEval_ConstantsStageVarAndPlayerIDExist(t *testing.T) {
	ctx := Context{}
	for expr, want := range map[string]float64{
		"floor(pi * 1000)": 3141, "floor(e * 1000)": 2718,
		`stagevar(info.name) = "Training"`: 0, `stagevar(info.author) != "x"`: 1, "stagevar(camera.boundleft)": 0,
		"playeridexist(56)": 1, "playeridexist(57)": 1, "playeridexist(99)": 0,
	} {
		if got := evalFloat(t, expr, ctx); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
}

// char.go:5952-5957, 6597-6620: the win family reads the decided round.
func TestEval_RoundEndTriggers(t *testing.T) {
	ko := Context{RoundEnd: RoundEnd{Decided: true, Winner: 0, KO: true}}
	ko.Health = 1000
	lost := ko
	lost.Side = 1
	draw := Context{RoundEnd: RoundEnd{Decided: true, Draw: true}}
	fighting := Context{}
	cases := []struct {
		ctx  Context
		want map[string]float64
	}{
		{ko, map[string]float64{"win": 1, "winko": 1, "wintime": 0, "winperfect": 1, "lose": 0, "drawgame": 0}},
		{lost, map[string]float64{"win": 0, "lose": 1, "loseko": 1, "losetime": 0}},
		{draw, map[string]float64{"win": 0, "lose": 0, "drawgame": 1}},
		{fighting, map[string]float64{"win": 0, "lose": 0, "drawgame": 0, "matchover": 0}},
	}
	for i, c := range cases {
		for expr, want := range c.want {
			if got := evalFloat(t, expr, c.ctx); got != want {
				t.Errorf("case %d %s = %v, want %v", i, expr, got, want)
			}
		}
	}
}

// playerid(id) and player(n) name the fighter itself or its opponent.
func TestEval_PlayerRedirections(t *testing.T) {
	ctx := Context{}
	ctx.Health = 700
	ctx.P2 = Opponent{Valid: true}
	ctx.P2.Side, ctx.P2.Health = 1, 400
	for expr, want := range map[string]float64{
		"playerid(56),life": 700, "playerid(57),life": 400, "playerid(3),life": 0,
		"player(1),life": 700, "player(2),life": 400, "player(4),life": 0,
	} {
		if got := evalFloat(t, expr, ctx); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
}

// Polnareff: "root,p2movetype = H" compares on the root, which faces the
// same opponent.
func TestEval_RedirectedTypeComparison(t *testing.T) {
	ctx := Context{IsHelper: true}
	ctx.Root = Opponent{Valid: true, MoveType: 'A'}
	ctx.P2 = Opponent{Valid: true, MoveType: 'H'}
	for expr, want := range map[string]float64{
		"root,p2movetype = H": 1, "root,movetype = A": 1, "root,movetype != A": 0, "root,statetype = S": 0,
	} {
		if got := evalFloat(t, expr, ctx); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
}

// compiler.go:3043-3052: a malformed HitDefAttr comparison reads false.
func TestEval_HitDefAttrWithoutStateLettersIsFalse(t *testing.T) {
	ctx := Context{HitDefAttr: 0x7fffffff}
	for _, expr := range []string{"HitDefAttr = ,AT", "hitdefattr=,AT,AP || 0"} {
		if got := evalFloat(t, expr, ctx); got != 0 {
			t.Errorf("%s = %v, want 0", expr, got)
		}
	}
}

// compiler.go:1285-1290: "ProjHit(1201)" is gibberish read as 0 (Hope).
func TestEval_ProjHitWithAnIDInParenthesesIsZero(t *testing.T) {
	ctx := Context{GameTime: 50, ProjContacts: []ProjContact{{ID: 1201, HitTick: 49}}}
	for expr, want := range map[string]float64{"projhit(1201) >= 1": 0, "projhit(1201) = 0": 1, "projcontact(5)": 0} {
		if got := evalFloat(t, expr, ctx); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
}
