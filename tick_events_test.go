package engine

import (
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// The tick reports each fighter's visual controllers of that tick, and only
// that tick's.
func TestTick_ReportsTheVisualEventsOfTheTick(t *testing.T) {
	states := stateWith(ctrl("PalFX", "time", "5"), ctrl("Width", "edge", "12, 3"))
	states[0] = cns.StateDef{Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: []cns.Controller{
		{Type: "PalFX", Triggers: []string{"time = 0"}, Parameters: map[string]string{"time": "5"}},
		{Type: "Width", Triggers: []string{"time = 0"}, Parameters: map[string]string{"edge": "12, 3"}},
	}}
	prog := FighterProgram{States: states}
	result := tickWithP1Program(t, prog, match.FighterState{Health: 1000}, 0)
	ev := result.Events[match.SideP1]
	if len(ev) != 1 || ev[0].Type != "PalFX" || len(result.Events[match.SideP2]) != 0 {
		t.Fatalf("events = %+v, want P1's PalFX", result.Events)
	}
	if w := result.Fighters[match.SideP1].Context.WidthEdge; w != [2]float64{12, 3} {
		t.Errorf("width edge = %v, want 12 3", w)
	}
	next, err := Tick(result.State, [2]FighterProgram{prog, {States: idleStates()}}, result.Fighters, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Tick: 2, ComboWindow: 60})
	if err != nil {
		t.Fatal(err)
	}
	if len(next.Events[match.SideP1]) != 0 {
		t.Errorf("next tick events = %+v, want none: events last one tick", next.Events)
	}
	if w := next.Fighters[match.SideP1].Context.WidthEdge; w != ([2]float64{}) {
		t.Errorf("next tick width edge = %v, want reset", w)
	}
}
