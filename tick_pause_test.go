package engine

import (
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/match"
)

// counterProgram's state 0 counts its ticks in var(0), and runs extra.
func counterProgram(extra ...cns.Controller) FighterProgram {
	controllers := append([]cns.Controller{ctrlWith("VarAdd", "1", map[string]string{"var(0)": "1"})}, extra...)
	return FighterProgram{States: map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, Controllers: controllers}},
		Animations: []air.Animation{{Number: 0, Frames: []air.Frame{{Time: -1}}}}}
}

// char.go:9384-9440 and 2791-2802: a SuperPause asked on tick 1 starts on
// tick 2 for its time; the opponent stands frozen through it, the pauser
// moves only for its movetime; it darkens, adds power and shows its effect.
func TestTick_SuperPauseFreezesTheOthers(t *testing.T) {
	pauser := counterProgram(ctrlWith("SuperPause", "Time = 0", map[string]string{"time": "10", "movetime": "3", "poweradd": "500"}))
	state, progs, rts := projMatch(t, pauser, 900)
	progs[1] = counterProgram()
	rt2, err := progs[1].NewRuntime(state.Fighters[1])
	if err != nil {
		t.Fatal(err)
	}
	rts[1] = rt2
	_, all := runProj(t, state, progs, rts, 14)
	first := all[0]
	if first.State.SuperTime != 10 || !first.State.SuperDarken || first.State.Fighters[0].Power != 500 {
		t.Fatalf("after tick 1: super time %d darken %v power %d, want 10, true, 500", first.State.SuperTime, first.State.SuperDarken, first.State.Fighters[0].Power)
	}
	if n := len(first.Explods[match.SideP1]); n != 1 || !first.Explods[match.SideP1][0].Common || first.Explods[match.SideP1][0].Anim != 100 {
		t.Errorf("super effect = %+v, want the common fightfx 100", first.Explods[match.SideP1])
	}
	opp := func(i int) int { return all[i].Fighters[match.SideP2].Context.Vars[0] }
	own := func(i int) int { return all[i].Fighters[match.SideP1].Context.Vars[0] }
	if opp(10) != opp(0) {
		t.Errorf("opponent counted %d ticks during the pause, want 0", opp(10)-opp(0))
	}
	if own(10)-own(0) != 3 {
		t.Errorf("pauser counted %d ticks during the pause, want its movetime 3", own(10)-own(0))
	}
	if opp(13) == opp(10) {
		t.Error("the opponent never moved again after the pause")
	}
}
