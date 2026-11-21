package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// The real kfm's intro (state 191, kfm.cns:193-212) kicks two wood pieces,
// explods 191 and 192 at pos 260,-90 and 60,-70 with velocity and accel,
// removed after 48 and 35 ticks.
func TestE2E_KfmsIntroWoodPiecesAreExplods(t *testing.T) {
	m := newE2EMatchAt(t, loadReferenceProgram(t, kfmCnsDef), 191)
	var first []Explod
	seen := map[int]int{}
	for i := 0; i < 120; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			t.Fatal(err)
		}
		es := m.runtimes[match.SideP1].Explods
		for _, e := range es {
			seen[e.Anim]++
		}
		if first == nil && len(es) > 0 {
			first = append([]Explod(nil), es...)
		}
	}
	if len(first) == 0 || first[0].Anim != 191 {
		t.Fatalf("first explods = %+v, want wood 191", first)
	}
	w := first[0]
	if w.Position.X != e2eP1X+260 || w.Position.Y != 90 {
		t.Errorf("wood 191 at %v,%v, want %v,90 (pos 260,-90 from kfm facing right)", w.Position.X, w.Position.Y, e2eP1X+260)
	}
	if seen[191] != 48 {
		t.Errorf("wood 191 lived %d ticks, want 48 (removetime)", seen[191])
	}
	if seen[192] != 35 {
		t.Errorf("wood 192 lived %d ticks, want 35 (removetime)", seen[192])
	}
}
