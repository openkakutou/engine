package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// The real kfm's Fast Kung Fu Palm (kfm.cns 1020) throws its opponent into
// kfm's custom state 1026, whose "No scroll" ScreenBound (value = 1,
// movecamera = 0,1) near a screen edge keeps the thrown kfm on screen but
// stops the camera from following it across.
func TestE2E_KfmsPalmStopsTheCameraFollowingTheThrownFighter(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	m := newE2EMatchAt(t, prog, 1020)
	m.state.Fighters[match.SideP1].Position.X = 880
	m.runtimes[match.SideP1].Context.Position.X = 880
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: 910}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := prog.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = prog, rt2, p2
	noScroll := false
	for i := 0; i < 120 && !noScroll; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			t.Fatal(err)
		}
		c := m.runtimes[match.SideP2].Context
		if c.StateNo == 1026 && c.InOpponentStates && c.MoveCamera == [2]bool{false, true} {
			noScroll = true
			if !c.ScreenBound {
				t.Error("No scroll's value = 1 left the thrown kfm unbound")
			}
			if c.Position.X > 1000 {
				t.Errorf("thrown kfm at %v, past the screen edge 1000", c.Position.X)
			}
		}
	}
	if !noScroll {
		t.Fatal("the thrown kfm never reached 1026's No scroll")
	}
}
