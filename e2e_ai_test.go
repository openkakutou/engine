package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// kfm_zss's AI.zss (state -1, appended after command.zss's) runs when the
// game gives the fighter an AI level: with no input at all, a level-8 kfm_zss
// facing an idle kfm acts on its own within 10 seconds, and never fails.
func TestE2E_KfmZssWithAnAILevel_ItsOwnAIPlaysWithoutInput(t *testing.T) {
	ai := loadReferenceProgram(t, kfmZssDef)
	m := newE2EMatch(t, ai)
	m.runtimes[match.SideP1].Context.AILevel = 8
	defender := loadReferenceProgram(t, kfmCnsDef)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 60}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := defender.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = defender, rt2, p2
	states := map[int]bool{}
	for range 600 {
		if err := m.step(input.TickInput{}); err != nil {
			t.Fatalf("tick %d: %v", m.tick, err)
		}
		no, _, _ := m.p1()
		states[no] = true
	}
	if len(states) < 2 {
		t.Fatalf("states = %v: the AI never acted", states)
	}
}
