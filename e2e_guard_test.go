package engine

import (
	"github.com/openkakutou/engine/v2/combat"
	"github.com/openkakutou/engine/v2/evaluator"
	"slices"
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// guardScenario: P1 (attackerDef) performs script; P2 (kfm, facing left, 40
// units ahead) holds back (right) the whole time, plus down when crouch.
// It returns P2's states and its life at the end.
func guardScenario(t *testing.T, attackerDef string, script []input.TickInput, crouch bool) ([]int, int) {
	return guardScenarioAgainst(t, attackerDef, kfmCnsDef, script, crouch)
}

func guardScenarioAgainst(t *testing.T, attackerDef, defenderDef string, script []input.TickInput, crouch bool) ([]int, int) {
	t.Helper()
	m := newE2EMatch(t, loadReferenceProgram(t, attackerDef))
	defender := loadReferenceProgram(t, defenderDef)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 40}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := defender.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = defender, rt2, p2
	back := input.TickInput{Right: true, Down: crouch}
	var states []int
	for i := range 90 {
		var in [2]input.TickInput
		if i < len(script) {
			in[0] = script[i]
		}
		in[1] = back
		m.tick++
		res, err := Tick(m.state, m.programs, m.runtimes, in, TickConfig{Bounds: testBounds(), Tick: m.tick, ComboWindow: 60})
		if err != nil {
			t.Fatalf("tick %d: %v", m.tick, err)
		}
		m.state, m.runtimes = res.State, res.Fighters
		no := m.state.Fighters[match.SideP2].StateNo
		if len(states) == 0 || states[len(states)-1] != no {
			states = append(states, no)
		}
	}
	return states, m.state.Fighters[match.SideP2].Health
}

func hitStates(states []int) bool {
	return slices.ContainsFunc(states, func(n int) bool { return n >= 5000 && n < 5200 })
}

var (
	lightKick = []input.TickInput{{Buttons: map[string]bool{"a": true}}}
	sweep     = []input.TickInput{{Down: true}, {Down: true}, {Down: true}, {Down: true, Buttons: map[string]bool{"b": true}}}
)

// char.go:11466-11503 (canguard, guardflag against the stance) and
// 13163-13173 (guard hit states): a standing guard against kfm_zss's light
// kick (guardflag MA, damage 26, guard damage 0) enters 150 and loses no life.
func TestE2E_StandingGuardBlocksTheLightKick(t *testing.T) {
	states, life := guardScenario(t, kfmZssDef, lightKick, false)
	if !slices.Contains(states, 150) || hitStates(states) {
		t.Errorf("defender states = %v, want the guard hit 150 and no get-hit state", states)
	}
	if life != 1000 {
		t.Errorf("defender life = %d, want 1000 (guard damage 0)", life)
	}
}

// kfm's sweep (state 440) has guardflag L: crouching guards it (152), standing
// does not.
func TestE2E_CrouchingGuardBlocksTheSweepAndStandingDoesNot(t *testing.T) {
	states, life := guardScenario(t, kfmCnsDef, sweep, true)
	if !slices.Contains(states, 152) || hitStates(states) || life != 1000 {
		t.Errorf("crouching: states %v life %d, want 152, no get-hit, 1000", states, life)
	}
	states, life = guardScenario(t, kfmCnsDef, sweep, false)
	if !hitStates(states) || life >= 1000 {
		t.Errorf("standing: states %v life %d, want hit by the low sweep", states, life)
	}
}

// The same guard with Ikemen GO's common1.cns.zss (kfm_zss defending).
func TestE2E_StandingGuardBlocksTheLightKickWithCommon1Zss(t *testing.T) {
	states, life := guardScenarioAgainst(t, kfmZssDef, kfmZssDef, lightKick, false)
	if !slices.Contains(states, 150) || hitStates(states) || life != 1000 {
		t.Errorf("defender states = %v life %d, want 150, no get-hit, 1000", states, life)
	}
}

// char.go:11493-11499 with computeDamage's kill flag (8907-8915): a guard
// that would not survive the chip is not a guard, unless guard.kill is 0,
// which caps the chip at life - 1.
func TestGuards_ChipDamageAndGuardKill(t *testing.T) {
	attacker := evaluator.Context{}
	defender := evaluator.Context{Guarding: true}
	defender.StateType, defender.Health = 'S', 4
	params := combat.ParseHitDef(map[string]string{"guardflag": "MA"})
	if guards(&attacker, &defender, params, 4) {
		t.Error("4 chip on 4 life with guard.kill 1: the guard fails, the hit lands")
	}
	if !guards(&attacker, &defender, params, 3) {
		t.Error("3 chip on 4 life: guarded")
	}
	params = combat.ParseHitDef(map[string]string{"guardflag": "MA", "guard.kill": "0"})
	if !guards(&attacker, &defender, params, 9) {
		t.Error("guard.kill 0: the chip cannot kill, so the guard holds")
	}
	if got := guardChip(9, 4, params); got != 3 {
		t.Errorf("chip 9 on 4 life with guard.kill 0 = %d, want 3", got)
	}
}
