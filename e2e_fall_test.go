package engine

import (
	"fmt"
	"slices"
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// scenarioSweepKnocksDown: kfm (P1) crouches then sweeps (down + b, state
// 440: ground.type Trip, fall 1, kfm.cns:654-706) a defender standing 40
// units ahead. The defender goes through common1's get-hit and fall states
// down to lying (5110), gets up (5120) and is back standing in state 0 with
// control (common1.cns:873-1530, common1.cns.zss). The trip's lie down (5070)
// raises its defence by kfm's fall.defence_up 50 until it stands again
// (char.go:12949-12953, 12671-12672).
func scenarioSweepKnocksDown(t *testing.T, defender FighterProgram) ([]int, error) {
	m := newE2EMatch(t, loadReferenceProgram(t, kfmCnsDef))
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 40}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := defender.NewRuntime(p2)
	if err != nil {
		return nil, err
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = defender, rt2, p2
	down := input.TickInput{Down: true}
	sweep := input.TickInput{Down: true, Buttons: map[string]bool{"b": true}}
	script := []input.TickInput{down, down, down, sweep}
	var states []int
	lyingDefence := 0.0
	for i := range 400 {
		in := input.TickInput{}
		if i < len(script) {
			in = script[i]
		}
		if err := m.step(in); err != nil {
			return states, err
		}
		no := m.state.Fighters[match.SideP2].StateNo
		if no == 5110 {
			lyingDefence = m.runtimes[match.SideP2].Context.FinalDefence()
		}
		if len(states) == 0 || states[len(states)-1] != no {
			states = append(states, no)
		}
	}
	for _, want := range []int{5110, 5120} {
		if !slices.Contains(states, want) {
			return states, fmt.Errorf("defender states %v never reach %d", states, want)
		}
	}
	if !slices.ContainsFunc(states, func(n int) bool { return n >= 5000 && n < 5100 }) {
		return states, fmt.Errorf("defender states %v never enter a get-hit state", states)
	}
	last := m.runtimes[match.SideP2].Context
	if last.StateNo != 0 || !last.Ctrl {
		return states, fmt.Errorf("defender ends in state %d ctrl %v (states %v), want standing 0 with control", last.StateNo, last.Ctrl, states)
	}
	if lyingDefence != 1.5 {
		return states, fmt.Errorf("defender defence lying down = %v, want 1.5 after the trip", lyingDefence)
	}
	if d := last.FinalDefence(); d != 1 {
		return states, fmt.Errorf("defender defence once standing = %v, want 1", d)
	}
	if h := m.state.Fighters[match.SideP2].Health; h >= 1000 {
		return states, fmt.Errorf("defender life %d: the sweep never hit", h)
	}
	return states, nil
}

func TestE2E_KfmSweep_KnocksDownKfmThroughCommon1Cns(t *testing.T) {
	if states, err := scenarioSweepKnocksDown(t, loadReferenceProgram(t, kfmCnsDef)); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("defender states: %v", states)
	}
}

func TestE2E_KfmSweep_KnocksDownKfmZssThroughCommon1Zss(t *testing.T) {
	if states, err := scenarioSweepKnocksDown(t, loadReferenceProgram(t, kfmZssDef)); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("defender states: %v", states)
	}
}
