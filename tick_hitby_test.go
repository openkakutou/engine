package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// A defender under NotHitBy SCA (every state type) is not hit by kfm_zss's
// light kick, which hits it otherwise (scenarioLightKickHitsKfm); the slot
// is refreshed every tick, as common1's states do.
func TestTick_NotHitByKeepsTheDefenderFromBeingHit(t *testing.T) {
	m := newE2EMatch(t, loadReferenceProgram(t, kfmZssDef))
	defender := loadReferenceProgram(t, kfmCnsDef)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 40}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := defender.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = defender, rt2, p2
	sca, err := evaluator.ParseAttr("SCA", false)
	if err != nil {
		t.Fatal(err)
	}
	in := input.TickInput{Buttons: map[string]bool{"a": true}}
	for range 30 {
		m.runtimes[match.SideP2].Context.HitBy[0] = evaluator.HitBySlot{Attr: sca, Time: 2, Not: true}
		if err := m.step(in); err != nil {
			t.Fatal(err)
		}
		in = input.TickInput{}
	}
	if h := m.state.Fighters[match.SideP2].Health; h != 1000 {
		t.Errorf("defender life = %d, want 1000: NotHitBy SCA lets nothing hit", h)
	}
}

// char.go:12365-12371: each tick counts the slots down and clears one at 0.
func TestTick_HitBySlotsCountDown(t *testing.T) {
	m := newE2EMatch(t, loadReferenceProgram(t, kfmCnsDef))
	m.runtimes[match.SideP1].Context.HitBy[2] = evaluator.HitBySlot{Attr: 1, Time: 2}
	for range 2 {
		if err := m.step(input.TickInput{}); err != nil {
			t.Fatal(err)
		}
	}
	if s := m.runtimes[match.SideP1].Context.HitBy[2]; s != (evaluator.HitBySlot{}) {
		t.Errorf("slot after 2 ticks = %+v, want cleared", s)
	}
}

// char.go:8897: a defence doubled by DefenceMulSet halves the damage:
// kfm_zss's light kick, 26 (scenarioLightKickHitsKfm), takes 13.
func TestTick_DefenceMultiplierScalesTheDamageTaken(t *testing.T) {
	m := newE2EMatch(t, loadReferenceProgram(t, kfmZssDef))
	defender := loadReferenceProgram(t, kfmCnsDef)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 40}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := defender.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	rt2.Context.CustomDefence = 2
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = defender, rt2, p2
	in := input.TickInput{Buttons: map[string]bool{"a": true}}
	for range 30 {
		if err := m.step(in); err != nil {
			t.Fatal(err)
		}
		in = input.TickInput{}
	}
	if h := m.state.Fighters[match.SideP2].Health; h != 1000-13 {
		t.Errorf("defender life = %d, want %d", h, 1000-13)
	}
}
