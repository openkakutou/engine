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

// A seeded random-input match of the corpus Ryu (SVC) against a real kfm
// runs without an engine error: every trigger and controller his states
// reach under real play is known (the native game stops the match on one).
func TestE2E_RyuSurvivesARandomMatch(t *testing.T) {
	for _, seed := range []uint32{12345, 777, 2026} {
		randomRyuMatch(t, seed)
	}
}

func randomRyuMatch(t *testing.T, start uint32) {
	t.Helper()
	seed := start
	path := corpusChar(t, "Street Fighter/Ryu (SVC)/Ryu_SS.def")
	prog, _, err := LoadFighterProgram(loadChar(t, path), path, referenceDir)
	if err != nil {
		t.Fatal(err)
	}
	m := newE2EMatch(t, prog)
	defender := loadReferenceProgram(t, kfmCnsDef)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 120}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := defender.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = defender, rt2, p2
	next := func() uint32 {
		seed = seed*1664525 + 1013904223
		return seed >> 16
	}
	buttons := []string{"a", "b", "c", "x", "y", "z"}
	var held [2]input.TickInput
	unsupported := map[string]bool{}
	for i := 0; i < 3000; i++ {
		for side := range held {
			if next()%6 == 0 {
				r := next()
				in := input.TickInput{Up: r&1 != 0 && r&2 == 0, Down: r&4 != 0, Left: r&8 != 0, Right: r&16 != 0 && r&8 == 0, Buttons: map[string]bool{}}
				if next()%3 == 0 {
					in.Buttons[buttons[next()%6]] = true
				}
				held[side] = in
			}
		}
		m.tick++
		res, err := Tick(m.state, m.programs, m.runtimes, held, TickConfig{Bounds: testBounds(), Gravity: e2eGravity, Tick: m.tick, ComboWindow: 60})
		if err != nil {
			t.Fatalf("seed %d tick %d: %v", start, m.tick, err)
		}
		m.state, m.runtimes = res.State, res.Fighters
		for _, u := range res.Unsupported[match.SideP1] {
			unsupported[u] = true
		}
		if res.Round.Outcome != 0 {
			break
		}
	}
	t.Logf("seed %d: unsupported controllers seen: %v", start, unsupported)
}

// Down, down-forward, forward and x make the corpus Ryu (SVC) throw his
// hadouken: his command buffer helper (9999, keyctrl 1, Ryu-2.cns) and his
// state -1 read the command, and he enters state 1000, which throws helper
// 1010.
func TestE2E_RyusHadoukenFromItsCommand(t *testing.T) {
	path := corpusChar(t, "Street Fighter/Ryu (SVC)/Ryu_SS.def")
	prog, _, err := LoadFighterProgram(loadChar(t, path), path, referenceDir)
	if err != nil {
		t.Fatal(err)
	}
	m := newE2EMatch(t, prog)
	var seq []input.TickInput
	seq = append(seq, make([]input.TickInput, 30)...)
	for _, in := range []input.TickInput{{Down: true}, {Down: true, Right: true}, {Right: true}} {
		seq = append(seq, in, in, in)
	}
	seq = append(seq, input.TickInput{Right: true, Buttons: map[string]bool{"x": true}})
	seq = append(seq, make([]input.TickInput, 40)...)
	entered, thrown := false, false
	for _, in := range seq {
		if err := m.step(in); err != nil {
			t.Fatal(err)
		}
		entered = entered || m.state.Fighters[match.SideP1].StateNo == 1000
		for _, h := range m.runtimes[match.SideP1].Helpers {
			thrown = thrown || h.Context.HelperID == 1010
		}
	}
	if !entered || !thrown {
		t.Errorf("state 1000 entered %v, hadouken thrown %v; want both", entered, thrown)
	}
}
