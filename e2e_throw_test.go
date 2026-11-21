package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// The real kfm's throw (kfm.cns states 800, 810, 820, 821): its HitDef
// (attr S, NT; hitflag M-; p1stateno 810; p2stateno 820) sends the thrown
// kfm into the thrower's state 820, a custom state, where TargetBind holds
// it until TargetLifeAdd -78 and TargetState 821 throw it; it falls back
// into its own states (SelfState 5100) and gets up, while the thrower goes
// back to state 0.
func TestE2E_KfmThrowsKfm(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	m := newE2EMatchAt(t, prog, 800)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 30}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := prog.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = prog, rt2, p2
	saw810, saw820Custom, saw821, backHome := false, false, false, false
	for i := 0; i < 240; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			t.Fatal(err)
		}
		p1, p2 := m.runtimes[match.SideP1].Context, m.runtimes[match.SideP2].Context
		saw810 = saw810 || p1.StateNo == 810
		saw820Custom = saw820Custom || (p2.StateNo == 820 && p2.InOpponentStates)
		saw821 = saw821 || (p2.StateNo == 821 && p2.InOpponentStates)
		backHome = backHome || (saw821 && !p2.InOpponentStates)
	}
	if !saw810 || !saw820Custom || !saw821 || !backHome {
		t.Fatalf("thrower in 810 %v, thrown in custom 820 %v, 821 %v, back in its own states %v; want all", saw810, saw820Custom, saw821, backHome)
	}
	if h := m.state.Fighters[match.SideP2].Health; h != 1000-78 {
		t.Errorf("thrown kfm's life = %d, want 922 (TargetLifeAdd -78)", h)
	}
	if s := m.state.Fighters[match.SideP1].StateNo; s != 0 {
		t.Errorf("thrower state after the throw = %d, want 0", s)
	}
}
