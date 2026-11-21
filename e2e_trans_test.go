package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// The corpus Ranma (CFJ) draws state 6011 with "trans = addalpha, alpha =
// 256,256" each tick: the fighter is drawn addalpha, its alpha clamped to
// 255 (paramTrans).
func TestE2E_RanmasTransHoldsInItsState(t *testing.T) {
	path := corpusChar(t, "Ranma/Ranma/Ranma.def")
	prog, _, err := LoadFighterProgram(loadChar(t, path), path, referenceDir)
	if err != nil {
		t.Fatal(err)
	}
	m := newE2EMatchAt(t, prog, 6011)
	if err := m.step(input.TickInput{}); err != nil {
		t.Fatal(err)
	}
	c := m.runtimes[match.SideP1].Context
	if c.StateNo != 6011 || c.Trans != "addalpha" || c.TransAlpha != [2]int{255, 255} {
		t.Fatalf("state %d trans %q alpha %v, want 6011 addalpha 255,255", c.StateNo, c.Trans, c.TransAlpha)
	}
}
