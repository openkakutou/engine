package engine

import (
	"strings"
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/stage"
)

// Item 063: player push, Ikemen GO's CharList.pushDetection (char.go:14551-
// 14779, commit 54d4148a). Fighters without Constants read Ikemen GO's
// default size (char.go:411-418): ground front/back 16, air 12, height 60,
// weight 100, pushfactor 1.

// pushBody is a Clsn2 wide enough for any two fighters 40 apart to touch:
// Ikemen GO pushes only when the Clsn2 boxes overlap too (char.go:14641-14645).
var pushBody = air.Frame{Clsn2: []air.ClsnBox{{Left: -20, Top: -60, Right: 20, Bottom: 0}}}

func pushCtx(x, y float64, facing match.Facing, stateType byte) evaluator.Context {
	return evaluator.Context{
		FighterState: match.FighterState{Position: match.Position{X: x, Y: y}, Facing: facing, Health: 1000},
		StateType:    stateType, MoveType: 'I', PlayerPush: true, PushAffectTeam: 1,
	}
}

func runPush(p1, p2 evaluator.Context, bounds *stage.StageBoundaries) (float64, float64) {
	pushFighters(&p1, &p2, [2]air.Frame{pushBody, pushBody}, bounds)
	return p1.Position.X, p2.Position.X
}

func wantXs(t *testing.T, gotP1, gotP2, wantP1, wantP2 float64) {
	t.Helper()
	if !approx(gotP1, wantP1) || !approx(gotP2, wantP2) {
		t.Errorf("X after push = %v, %v, want %v, %v", gotP1, gotP2, wantP1, wantP2)
	}
}

// Size boxes [-16, 16] at 0 and [4, 36] at 20 overlap by 12; equal weights
// and pushfactors split it: each moves 6 away (char.go:14685-14691, 14760-14779).
func TestPushFighters_OverlappingStandingFighters_SplitTheOverlap(t *testing.T) {
	p1, p2 := runPush(pushCtx(0, 0, match.FacingRight, 'S'), pushCtx(20, 0, match.FacingLeft, 'S'), testBounds())
	wantXs(t, p1, p2, -6, 26)
}

// P1 on the right of P2: the push goes the other way (distX sign,
// char.go:14731-14779).
func TestPushFighters_FirstFighterOnTheRight_IsPushedRight(t *testing.T) {
	p1, p2 := runPush(pushCtx(20, 0, match.FacingLeft, 'S'), pushCtx(0, 0, match.FacingRight, 'S'), testBounds())
	wantXs(t, p1, p2, 26, -6)
}

// Ground front 16 + front 16 = 32: at 32 the boxes only touch (overlap 0),
// a no-op (char.go:14636-14639); further apart nothing happens either.
func TestPushFighters_TouchingOrApart_DoesNotMove(t *testing.T) {
	for _, x := range []float64{32, 50} {
		p1, p2 := runPush(pushCtx(0, 0, match.FacingRight, 'S'), pushCtx(x, 0, match.FacingLeft, 'S'), testBounds())
		wantXs(t, p1, p2, 0, x)
	}
}

func TestPushFighters_EitherWithoutPlayerPush_DoesNotMove(t *testing.T) {
	for _, side := range []match.Side{match.SideP1, match.SideP2} {
		a, b := pushCtx(0, 0, match.FacingRight, 'S'), pushCtx(20, 0, match.FacingLeft, 'S')
		if side == match.SideP1 {
			a.PlayerPush = false
		} else {
			b.PlayerPush = false
		}
		p1, p2 := runPush(a, b, testBounds())
		wantXs(t, p1, p2, 0, 20)
	}
}

// Enemies are skipped when either pushes only its team (affectteam F, -1;
// char.go:14576-14580).
func TestPushFighters_AffectTeamFriendOnly_DoesNotPushTheEnemy(t *testing.T) {
	a, b := pushCtx(0, 0, match.FacingRight, 'S'), pushCtx(20, 0, match.FacingLeft, 'S')
	b.PushAffectTeam = -1
	p1, p2 := runPush(a, b, testBounds())
	wantXs(t, p1, p2, 0, 20)
}

// Size boxes overlap but the Clsn2 boxes do not: MUGEN-style, no push
// (char.go:14641-14645).
func TestPushFighters_ClsnTwoApart_DoesNotMove(t *testing.T) {
	a, b := pushCtx(0, 0, match.FacingRight, 'S'), pushCtx(20, 0, match.FacingLeft, 'S')
	thin := air.Frame{Clsn2: []air.ClsnBox{{Left: -2, Top: -60, Right: 2, Bottom: 0}}}
	pushFighters(&a, &b, [2]air.Frame{thin, thin}, testBounds())
	wantXs(t, a.Position.X, b.Position.X, 0, 20)
	pushFighters(&a, &b, [2]air.Frame{}, testBounds())
	wantXs(t, a.Position.X, b.Position.X, 0, 20)
}

// The higher priority stays; the other takes the overlap times its own
// pushfactor (char.go:14678-14683, 14765-14779).
func TestPushFighters_HigherPriority_StaysPut(t *testing.T) {
	a, b := pushCtx(0, 0, match.FacingRight, 'S'), pushCtx(20, 0, match.FacingLeft, 'S')
	a.PushPriority = 1
	p1, p2 := runPush(a, b, testBounds())
	wantXs(t, p1, p2, 0, 32)
	a.PushPriority, b.PushPriority = 0, 1
	p1, p2 = runPush(a, b, testBounds())
	wantXs(t, p1, p2, -12, 20)
}

// Weights decide the split, the average pushfactor the amount
// (char.go:14684-14691): P1 weighs 300, P2 100, pushfactor (0.5 + 1) / 2:
// P1 moves 12 * .75 * 100/400 = 2.25, P2 12 * .75 * 300/400 = 6.75.
func TestPushFighters_WeightAndPushFactor_ShareTheOverlap(t *testing.T) {
	heavy, err := evaluator.ParseConstants(strings.NewReader("[Size]\nweight = 300\npushfactor = 0.5\n"))
	if err != nil {
		t.Fatal(err)
	}
	a, b := pushCtx(0, 0, match.FacingRight, 'S'), pushCtx(20, 0, match.FacingLeft, 'S')
	a.Consts = heavy
	p1, p2 := runPush(a, b, testBounds())
	wantXs(t, p1, p2, -2.25, 26.75)
}

// P2 against the right bound cannot move: P1 takes the whole overlap times
// its own pushfactor (char.go:14787-14820).
func TestPushFighters_OneCornered_TheOtherTakesTheWholeOverlap(t *testing.T) {
	bounds := &stage.StageBoundaries{Left: -100, Right: 20}
	p1, p2 := runPush(pushCtx(0, 0, match.FacingRight, 'S'), pushCtx(20, 0, match.FacingLeft, 'S'), bounds)
	wantXs(t, p1, p2, -12, 20)
}

// An airborne fighter uses the air box (12 back and front, char.go:4449-
// 4459): [8, 32] at 20 against [-16, 16] overlaps by 8. Above the other's
// height (60) the boxes miss.
func TestPushFighters_AirborneFighter_UsesTheAirBoxAndTheHeight(t *testing.T) {
	p1, p2 := runPush(pushCtx(0, 0, match.FacingRight, 'S'), pushCtx(20, 30, match.FacingLeft, 'A'), testBounds())
	wantXs(t, p1, p2, -4, 24)
	p1, p2 = runPush(pushCtx(0, 0, match.FacingRight, 'S'), pushCtx(20, 70, match.FacingLeft, 'A'), testBounds())
	wantXs(t, p1, p2, 0, 20)
}

// Same X: the tie-break pushes a hit fighter back along its facing
// (char.go:14700-14722).
func TestPushFighters_SamePosition_TieBreakOnMoveType(t *testing.T) {
	a, b := pushCtx(0, 0, match.FacingRight, 'S'), pushCtx(0, 0, match.FacingLeft, 'S')
	a.MoveType = 'H' // tmp = -c.facing = -1: P1 goes left
	p1, p2 := runPush(a, b, testBounds())
	wantXs(t, p1, p2, -16, 16)
}

// pushProgram is a standing fighter whose animation 0 has pushBody's Clsn2.
func pushProgram(ctrls ...cns.Controller) FighterProgram {
	return NewFighterProgram(map[int]cns.StateDef{0: standing(0, ctrls...)},
		[]air.Animation{{Number: 0, Frames: []air.Frame{{Time: -1, Clsn2: pushBody.Clsn2}}}}, cmd.CommandFile{})
}

func pushTick(t *testing.T, progs [2]FighterProgram, x1, x2 float64, ticks int) TickResult {
	t.Helper()
	p1 := match.FighterState{Side: match.SideP1, Facing: match.FacingRight, Position: match.Position{X: x1}, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Facing: match.FacingLeft, Position: match.Position{X: x2}, Health: 1000}
	rt1, err := progs[0].NewRuntime(p1)
	if err != nil {
		t.Fatal(err)
	}
	rt2, err := progs[1].NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	st, err := match.NewMatchState(1, 0, p1, p2)
	if err != nil {
		t.Fatal(err)
	}
	runtimes := [2]FighterRuntime{rt1, rt2}
	var res TickResult
	for tick := 1; tick <= ticks; tick++ {
		res, err = Tick(*st, progs, runtimes, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Tick: tick, ComboWindow: 60})
		if err != nil {
			t.Fatal(err)
		}
		st, runtimes = &res.State, res.Fighters
	}
	return res
}

// Tick pushes after both fighters moved, before hit detection
// (system.go:2734, char.go:14954-14960), on the match state and the runtimes.
func TestTick_OverlappingFighters_ArePushedApart(t *testing.T) {
	res := pushTick(t, [2]FighterProgram{pushProgram(), pushProgram()}, 0, 20, 1)
	wantXs(t, res.State.Fighters[match.SideP1].Position.X, res.State.Fighters[match.SideP2].Position.X, -6, 26)
	wantXs(t, res.Fighters[match.SideP1].Context.Position.X, res.Fighters[match.SideP2].Context.Position.X, -6, 26)
}

// PlayerPush value = 0 lasts the tick it runs in: Ikemen GO sets playerpush
// again before every unfrozen tick's states (char.go:12350-12361).
func TestTick_PlayerPushZero_LetsThemOverlapOnlyWhileItRuns(t *testing.T) {
	off := cns.Controller{Type: "PlayerPush", Triggers: []string{"Time < 2"}, Parameters: map[string]string{"value": "0"}}
	progs := [2]FighterProgram{pushProgram(off), pushProgram()}
	res := pushTick(t, progs, 0, 20, 2)
	wantXs(t, res.State.Fighters[match.SideP1].Position.X, res.State.Fighters[match.SideP2].Position.X, 0, 20)
	res = pushTick(t, progs, 0, 20, 3)
	wantXs(t, res.State.Fighters[match.SideP1].Position.X, res.State.Fighters[match.SideP2].Position.X, -6, 26)
}

// withoutPush adds PlayerPush value = 0 to every state of states: for the
// hand-built fixtures whose attacks reach less far than Ikemen GO's default
// size boxes (16 each side), where push would keep every hit from landing.
func withoutPush(states map[int]cns.StateDef) {
	off := cns.Controller{Type: "PlayerPush", Triggers: []string{"1"}, Parameters: map[string]string{"value": "0"}}
	for n, def := range states {
		def.Controllers = append([]cns.Controller{off}, def.Controllers...)
		states[n] = def
	}
}
