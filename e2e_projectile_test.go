package engine

import (
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// The real kfm's light punch (state 200, button x) throws a projectile with
// its own punch animation (200, Clsn1 on its second frame) across the stage
// into a real kfm 150 units away, out of the punch's own reach: the
// projectile hits as a HitDef (damage, the defender's get-hit state from
// common1), then disappears (projhitanim -1). The reference characters have
// no projectile of their own and the corpus ones throw theirs from helpers
// (Ryu SVC, Sakura CVS: item 029), so the controller is added to kfm here.
func TestE2E_AProjectileCrossesTheStageAndHits(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	punch := prog.States[200]
	punch.Controllers = append(append([]cns.Controller(nil), punch.Controllers...), cns.Controller{
		Type: "Projectile", Triggers: []string{"Time = 0"},
		Parameters: map[string]string{"projanim": "200", "velocity": "8, 0", "offset": "20, 0", "attr": "S, SP", "damage": "30", "animtype": "Light", "ground.velocity": "-4", "pausetime": "0, 8"},
	})
	prog.States[200] = punch
	m := newE2EMatch(t, prog)
	defender := loadReferenceProgram(t, kfmCnsDef)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 150}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := defender.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = defender, rt2, p2
	seen, hitState := 0, 0
	for i := 0; i < 40; i++ {
		var in [2]input.TickInput
		if i == 0 {
			in[0] = input.TickInput{Buttons: map[string]bool{"x": true}}
		}
		m.tick++
		res, err := Tick(m.state, m.programs, m.runtimes, in, TickConfig{Bounds: testBounds(), Gravity: e2eGravity, Tick: m.tick, ComboWindow: 60})
		if err != nil {
			t.Fatal(err)
		}
		m.state, m.runtimes = res.State, res.Fighters
		seen = max(seen, len(res.Projectiles[match.SideP1]))
		if s := res.State.Fighters[match.SideP2].StateNo; s >= 5000 && s < 5100 && hitState == 0 {
			hitState = s
		}
	}
	if seen != 1 {
		t.Errorf("live projectiles at most %d, want 1", seen)
	}
	if h := m.state.Fighters[match.SideP2].Health; h != 970 {
		t.Errorf("defender life %d, want 970 (damage 30)", h)
	}
	if hitState == 0 {
		t.Error("the defender never entered a get-hit state")
	}
	if n := len(m.runtimes[match.SideP1].Projectiles); n != 0 {
		t.Errorf("%d projectiles left after the hit, want 0", n)
	}
}
