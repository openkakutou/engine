package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// autoTurnMatch puts kfm standing in state 0 facing right, its opponent
// behind it.
func autoTurnMatch(t *testing.T) *e2eMatch {
	t.Helper()
	m := newE2EMatch(t, loadReferenceProgram(t, kfmCnsDef))
	m.state.Fighters[match.SideP2].Position.X = e2eP1X - 50
	m.state.Fighters[match.SideP2].Facing = match.FacingRight
	return m
}

// system.go:13953-13958 and char.go:6731-6745 autoTurn: a fighter with control
// in state 0 whose opponent is behind it turns, with the turn animation 5.
func TestTick_AFighterWhoseOpponentIsBehindTurnsAround(t *testing.T) {
	m := autoTurnMatch(t)
	if err := m.step(input.TickInput{}); err != nil {
		t.Fatal(err)
	}
	if f := m.state.Fighters[match.SideP1].Facing; f != match.FacingLeft {
		t.Fatalf("facing = %v, want left, toward the opponent", f)
	}
	if _, _, anim := m.p1(); anim != 5 {
		t.Errorf("anim = %d, want the turn animation 5", anim)
	}
}

func TestTick_NoAutoTurnKeepsTheFacing(t *testing.T) {
	m := autoTurnMatch(t)
	if err := m.runtimes[match.SideP1].Context.Asserted.Set("noautoturn", true); err != nil {
		t.Fatal(err)
	}
	if err := m.step(input.TickInput{}); err != nil {
		t.Fatal(err)
	}
	if f := m.state.Fighters[match.SideP1].Facing; f != match.FacingRight {
		t.Errorf("facing = %v, want right kept under noautoturn", f)
	}
}

func TestTick_AnOpponentInFrontDoesNotTurnTheFighter(t *testing.T) {
	m := newE2EMatch(t, loadReferenceProgram(t, kfmCnsDef))
	if err := m.step(input.TickInput{}); err != nil {
		t.Fatal(err)
	}
	if f := m.state.Fighters[match.SideP1].Facing; f != match.FacingRight {
		t.Errorf("facing = %v, want right", f)
	}
}
