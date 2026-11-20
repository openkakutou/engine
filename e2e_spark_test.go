package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// char.go:11313-11411 hitspark: kfm_zss's light kick (sparkno 0, sparkxy
// -10, -37, kfm.zss:343-359) reports a HitSpark at the target's front edge
// (size.ground.front 16) less the x offset, at the attacker's height plus the
// y offset (37 up); a guarded hit uses guard.sparkno (data.guard.sparkno 40).
func TestE2E_TheLightKickReportsItsHitSpark(t *testing.T) {
	for _, guard := range []bool{false, true} {
		m := newE2EMatch(t, loadReferenceProgram(t, kfmZssDef))
		defender := loadReferenceProgram(t, kfmCnsDef)
		p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 40}, Facing: match.FacingLeft, Health: 1000}
		rt2, err := defender.NewRuntime(p2)
		if err != nil {
			t.Fatal(err)
		}
		m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = defender, rt2, p2
		var spark *evaluator.ControllerEvent
		var defX float64
		var heard []evaluator.SoundEvent
		for i := 0; i < 30 && spark == nil; i++ {
			var in [2]input.TickInput
			if i == 0 {
				in[0] = input.TickInput{Buttons: map[string]bool{"a": true}}
			}
			if guard {
				in[1] = input.TickInput{Right: true}
			}
			m.tick++
			res, err := Tick(m.state, m.programs, m.runtimes, in, TickConfig{Bounds: testBounds(), Tick: m.tick, ComboWindow: 60})
			if err != nil {
				t.Fatal(err)
			}
			m.state, m.runtimes = res.State, res.Fighters
			heard = append(heard, res.Sounds[match.SideP1]...)
			for _, e := range res.Events[match.SideP1] {
				if e.Type == "HitSpark" {
					spark = &e
					defX = res.State.Fighters[match.SideP2].Position.X
				}
			}
		}
		if spark == nil {
			t.Fatalf("guard %v: no HitSpark reported", guard)
		}
		wantAnim := "0"
		if guard {
			wantAnim = "40"
		}
		if got := spark.Param("anim").Raw; got != wantAnim {
			t.Errorf("guard %v: spark anim %q, want %q", guard, got, wantAnim)
		}
		if x := spark.Param("x").Values; len(x) != 1 || x[0] != defX-6 {
			t.Errorf("guard %v: spark x %v, want %v (target %v)", guard, x, defX-6, defX)
		}
		if y := spark.Param("y").Values; len(y) != 1 || y[0] != 37 {
			t.Errorf("guard %v: spark y %v, want 37", guard, y)
		}
		// char.go:12100-12130: hitsound 5, 1 / guardsound 6, 0, read from the
		// common sound file (no prefix: "f").
		want := evaluator.SoundEvent{Group: 5, Sample: 1, Common: true, Channel: -1}
		if guard {
			want = evaluator.SoundEvent{Group: 6, Sample: 0, Common: true, Channel: -1}
		}
		found := false
		for _, s := range heard {
			found = found || s == want
		}
		if !found {
			t.Errorf("guard %v: sounds %v, want %v among them", guard, heard, want)
		}
	}
}
