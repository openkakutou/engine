package engine

import (
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/match"
)

// p1StateNoHitter's state 0 lands a HitDef with p1stateno 300 and pausetime
// 3,3 on its first tick; state 300 shows animation 300.
func p1StateNoHitter() FighterProgram {
	box := []air.ClsnBox{{Left: -5, Top: -50, Right: 5, Bottom: 0}}
	hit := ctrlWith("HitDef", "Time = 0", map[string]string{"attr": "S, NA", "damage": "10", "p1stateno": "300", "pausetime": "3, 3"})
	return FighterProgram{
		States: map[int]cns.StateDef{
			0:   {Number: 0, Type: cns.StateTypeStanding, MoveType: cns.MoveTypeAttack, Ctrl: true, Controllers: []cns.Controller{hit}},
			300: {Number: 300, Type: cns.StateTypeStanding, Anim: 300},
		},
		Animations: []air.Animation{
			{Number: 0, Frames: []air.Frame{{Time: -1, Clsn1: box}}},
			{Number: 300, Frames: []air.Frame{{Time: -1}}},
		},
	}
}

// char.go:12207-12209, 6943-6944, 13125-13129: p1stateno changes the
// attacker's state number and takes its control at the hit, but the state
// is entered (its header run: anim, hitcountpersist) once the hit pause is
// over; meanwhile the hit count, added at the end of the tick, reads 1.
func TestTick_P1StateNoIsEnteredOnceTheHitPauseIsOver(t *testing.T) {
	state, progs, rts := projMatch(t, p1StateNoHitter(), 5)
	_, all := runProj(t, state, progs, rts, 6)
	hit := all[0].Fighters[match.SideP1].Context
	if all[0].State.Fighters[match.SideP2].Health != 990 {
		t.Fatalf("defender life = %d, want 990 (the hit lands on tick 1)", all[0].State.Fighters[match.SideP2].Health)
	}
	if hit.StateNo != 300 || hit.Ctrl || hit.Anim != 0 || hit.HitCount != 1 {
		t.Errorf("hit tick: state %d ctrl %v anim %d hit count %d; want 300, false, 0 (not entered yet), 1", hit.StateNo, hit.Ctrl, hit.Anim, hit.HitCount)
	}
	paused := all[3].Fighters[match.SideP1].Context
	if paused.Anim != 0 || paused.HitCount != 1 {
		t.Errorf("tick 4 (hit pause): anim %d hit count %d; want 0, 1", paused.Anim, paused.HitCount)
	}
	after := all[4].Fighters[match.SideP1].Context
	if after.Anim != 300 || after.HitCount != 0 {
		t.Errorf("tick 5 (pause over): anim %d hit count %d; want 300, 0 (entered, no hitcountpersist)", after.Anim, after.HitCount)
	}
}
