package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// The real kfm's counter (kfm.cns state 1300): its ReversalDef (reversal.attr
// SA, AA; p1stateno 1310) reverses the other kfm's standing punch when their
// attack boxes meet: the reverser goes to 1310 untouched, the puncher's move
// counts as reversed (MoveReversed) and its HitDef is spent.
func TestE2E_KfmReversesAPunch(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	m := newE2EMatchAt(t, prog, 1300)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 40}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := prog.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = prog, rt2, p2
	saw1310, reversed := false, false
	for i := 0; i < 30; i++ {
		var in [2]input.TickInput
		if i == 0 {
			in[1] = input.TickInput{Buttons: map[string]bool{"x": true}}
		}
		m.tick++
		res, err := Tick(m.state, m.programs, m.runtimes, in, TickConfig{Bounds: testBounds(), Tick: m.tick, ComboWindow: 60})
		if err != nil {
			t.Fatal(err)
		}
		m.state, m.runtimes = res.State, res.Fighters
		saw1310 = saw1310 || m.runtimes[match.SideP1].Context.StateNo == 1310
		reversed = reversed || m.runtimes[match.SideP2].Context.MoveReversedTime > 0
	}
	if !saw1310 || !reversed {
		t.Errorf("reverser in 1310 %v, punch reversed %v; want both", saw1310, reversed)
	}
	if h := m.state.Fighters[match.SideP1].Health; h != 1000 {
		t.Errorf("reverser's life = %d, want 1000 (the punch reversed)", h)
	}
}
