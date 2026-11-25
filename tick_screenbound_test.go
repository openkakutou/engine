package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// char.go:10322-10336 xScreenBound: with the screen edges the game gives,
// a fighter walking toward one stops on it (widthEdge 0, the default).
func TestTick_AFighterWalkingIntoTheScreenEdgeStopsOnIt(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	m := newE2EMatch(t, prog)
	edge := m.state.Fighters[match.SideP1].Position.X + 10
	screen := [2]float64{edge - 300, edge}
	for range 60 {
		m.tick++
		result, err := Tick(m.state, m.programs, m.runtimes, [2]input.TickInput{match.SideP1: {Right: true}},
			TickConfig{Bounds: testBounds(), Screen: screen, Tick: m.tick, ComboWindow: 60})
		if err != nil {
			t.Fatal(err)
		}
		m.state, m.runtimes = result.State, result.Fighters
	}
	if x := m.state.Fighters[match.SideP1].Position.X; x != edge {
		t.Errorf("x after walking 60 ticks = %v, want stopped on the screen edge %v", x, edge)
	}
	if x := m.runtimes[match.SideP1].Context.Position.X; x != edge {
		t.Errorf("runtime x = %v, want %v", x, edge)
	}
}

func TestTick_WithoutScreenTheStageBoundsStillHold(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	m := newE2EMatch(t, prog)
	start := m.state.Fighters[match.SideP1].Position.X
	for range 30 {
		if err := m.step(input.TickInput{Right: true}); err != nil {
			t.Fatal(err)
		}
	}
	if x := m.state.Fighters[match.SideP1].Position.X; x <= start+10 {
		t.Errorf("x = %v: without a screen the fighter walks past %v", x, start+10)
	}
}
