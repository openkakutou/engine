package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// The reference kfm jumps forward over a close kfm (states 40, 50, 52):
// once he lands behind it, autoTurn (char.go) turns both toward each other.
func TestE2E_AJumpOverTheOpponentEndsFacingEachOther(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	m := newE2EMatch(t, prog)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 40}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := prog.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = prog, rt2, p2
	crossed := false
	for i := 0; i < 120; i++ {
		in := input.TickInput{}
		if i < 6 {
			in = input.TickInput{Up: true, Right: true}
		}
		if err := m.step(in); err != nil {
			t.Fatal(err)
		}
		a, b := m.state.Fighters[match.SideP1], m.state.Fighters[match.SideP2]
		crossed = crossed || a.Position.X > b.Position.X
	}
	a, b := m.state.Fighters[match.SideP1], m.state.Fighters[match.SideP2]
	if !crossed {
		t.Fatalf("P1 at %v never passed P2 at %v", a.Position.X, b.Position.X)
	}
	if a.Position.Y != 0 || b.Position.Y != 0 {
		t.Fatalf("not landed: %v / %v", a.Position, b.Position)
	}
	towardB := a.Facing == match.FacingRight == (b.Position.X > a.Position.X)
	towardA := b.Facing == match.FacingRight == (a.Position.X > b.Position.X)
	if !towardB || !towardA {
		t.Errorf("P1 at %v facing %v, P2 at %v facing %v: want both facing each other", a.Position.X, a.Facing, b.Position.X, b.Facing)
	}
}
