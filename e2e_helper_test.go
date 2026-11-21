package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// corpusChar is a character of the local real-file corpus
// (~/workspace/ikemen-quick-versus), whose files cannot be committed; a test
// using it is skipped where the corpus is absent.
func corpusChar(t *testing.T, rel string) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	path := filepath.Join(home, "workspace", "ikemen-quick-versus", "chars", rel)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("corpus character %s not found", rel)
	}
	return path
}

// Ryu (SVC, by Soy Sauce) throws his hadouken from a helper: state 1000
// spawns helper 1010 (Ryu-S.cns), which flies across the stage and hits a
// real kfm.
func TestE2E_RyusHadoukenHelperCrossesTheStageAndHits(t *testing.T) {
	path := corpusChar(t, "Street Fighter/Ryu (SVC)/Ryu_SS.def")
	prog, missing, err := LoadFighterProgram(loadChar(t, path), path, referenceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Fatalf("missing state files %v", missing)
	}
	m := newE2EMatchAt(t, prog, 1000)
	defender := loadReferenceProgram(t, kfmCnsDef)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 200}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := defender.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = defender, rt2, p2
	sawHadouken := false
	for i := 0; i < 120; i++ {
		m.tick++
		res, err := Tick(m.state, m.programs, m.runtimes, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Gravity: e2eGravity, Tick: m.tick, ComboWindow: 60})
		if err != nil {
			t.Fatalf("tick %d: %v", m.tick, err)
		}
		m.state, m.runtimes = res.State, res.Fighters
		for _, h := range res.Helpers[match.SideP1] {
			sawHadouken = sawHadouken || h.Context.HelperID == 1010
		}
	}
	if !sawHadouken {
		t.Fatal("helper 1010 (the hadouken) never appeared")
	}
	// damage = floor(90*root,fvar(0)*root,fvar(3)), 10 (Ryu-S.cns), both
	// fvars 1 before the hit (Ryu-2.cns state -2), Ryu's attack and kfm's
	// defence 100.
	if h := m.state.Fighters[match.SideP2].Health; h != 910 {
		t.Errorf("kfm's life = %d, want 910 (the hadouken's 90)", h)
	}
}
