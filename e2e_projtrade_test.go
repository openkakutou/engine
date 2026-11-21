package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// Two corpus Seiyas (Saint Seiya) throw their state 225 projectile (anim
// 1040, with hurt boxes) at each other: the projectiles meet and trade
// (char.go:2949-2966, 3080-3180), and an owner's projectile is cancelled.
func TestE2E_SeiyasProjectilesCancelEachOther(t *testing.T) {
	path := corpusChar(t, "Saint Seiya/Seiya/Seiya.def")
	prog, _, err := LoadFighterProgram(loadChar(t, path), path, referenceDir)
	if err != nil {
		t.Fatal(err)
	}
	m := newE2EMatchAt(t, prog, 225)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 250}, Facing: match.FacingLeft, Health: 1000, StateNo: 225}
	rt2, err := prog.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = prog, rt2, p2
	thrown, cancelled := false, false
	for i := 0; i < 150 && !cancelled; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			t.Fatal(err)
		}
		for side := range 2 {
			for _, p := range m.runtimes[side].Projectiles {
				thrown = true
				cancelled = cancelled || p.Status == ProjectileCancelled
			}
		}
	}
	if !thrown {
		t.Fatal("no projectile was thrown")
	}
	if !cancelled {
		t.Fatal("the projectiles never traded")
	}
	if m.state.Fighters[0].Health != 1000 || m.state.Fighters[1].Health != 1000 {
		t.Errorf("health %d / %d, want both untouched: the projectiles cancelled before reaching anyone", m.state.Fighters[0].Health, m.state.Fighters[1].Health)
	}
}
