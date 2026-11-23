package engine

import (
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/match"
)

// changeAnim2Match: P2 runs P1's state 900 (a custom state), which shows P1's
// animation 300 with ChangeAnim2, then records what the animation triggers
// read: var(1) AnimTime, var(2) SelfAnimExist(5), var(3) AnimExist(300),
// var(4) SelfAnimExist(300). P1 has animations 0 and 300 (10 ticks long),
// P2 animations 0 and 5.
func changeAnim2Match(t *testing.T) []TickResult {
	t.Helper()
	all := func(c cns.Controller) cns.Controller {
		c.Triggers = []string{"1"}
		return c
	}
	thrower := FighterProgram{
		States: map[int]cns.StateDef{
			0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true},
			900: {Number: 900, Type: cns.StateTypeStanding, MoveType: cns.MoveTypeHit, Controllers: []cns.Controller{
				all(ctrl("ChangeAnim2", "value", "300")),
				all(ctrl("VarSet", "var(1)", "AnimTime")),
				all(ctrl("VarSet", "var(2)", "SelfAnimExist(5)")),
				all(ctrl("VarSet", "var(3)", "AnimExist(300)")),
				all(ctrl("VarSet", "var(4)", "SelfAnimExist(300)")),
			}},
		},
		Animations: []air.Animation{{Number: 0, Frames: []air.Frame{{Time: -1}}}, {Number: 300, Frames: []air.Frame{{Time: 10}}}},
	}
	thrown := FighterProgram{
		States:     map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true}},
		Animations: []air.Animation{{Number: 0, Frames: []air.Frame{{Time: -1}}}, {Number: 5, Frames: []air.Frame{{Time: -1}}}},
	}

	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: 900}, Facing: match.FacingLeft, Health: 1000}
	state, progs, rts := projMatch(t, thrower, 900)
	progs[match.SideP2] = thrown
	r2, err := thrown.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	r2.Context.InOpponentStates = true
	r2.Context.StateNo = 900
	state.Fighters[match.SideP2].StateNo = 900
	rts[match.SideP2] = r2
	_, ticks := runProj(t, state, progs, rts, 2)
	return ticks
}

// char.go:5583-5602: after ChangeAnim2 the animation triggers read the
// owner's animation the same tick; SelfAnimExist still reads the
// character's own table, on the tick of the ChangeAnim2 and after.
func TestTick_ChangeAnim2IsReadAtOnceAndSelfAnimExistStaysOwn(t *testing.T) {
	for tick, res := range changeAnim2Match(t) {
		ctx := res.Fighters[match.SideP2].Context
		if ctx.StateNo != 900 || !ctx.AnimFromOpponent || ctx.Anim != 300 {
			t.Fatalf("tick %d: P2 state %d anim %d from opponent %v, want 900, 300, true", tick+1, ctx.StateNo, ctx.Anim, ctx.AnimFromOpponent)
		}
		want := [5]int{1: -10, 2: 1, 3: 1, 4: 0}
		for i := 1; i <= 4; i++ {
			if got := ctx.Vars[i]; got != want[i] {
				t.Errorf("tick %d: var(%d) = %d, want %d", tick+1, i, got, want[i])
			}
		}
	}
}
