package engine

import (
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/match"
)

// p2facing = -1 turns the defender to face the attacker's way
// (char.go:12194-12206): the turn is in the match state the tick returns,
// so the next tick starts from it. A synthetic pair: the real kfm only
// uses p2facing = 1, which the defender's own autoturn hides.
func TestTick_P2FacingTurnReachesTheMatchState(t *testing.T) {
	hit := ctrl("HitDef", "damage", "30", "attr", "S, NA", "p2facing", "-1")
	attacker := hitProgram()
	def := attacker.States[0]
	def.Controllers = stateWith(hit)[0].Controllers
	attacker.States[0] = def
	fighters := [2]match.FighterState{
		match.SideP1: {Facing: match.FacingRight, Health: 1000},
		match.SideP2: {Position: match.Position{X: 3}, Facing: match.FacingLeft, Health: 1000},
	}
	result := tickPair(t, [2]FighterProgram{attacker, hurtProgram()}, fighters, 0)
	if h := result.State.Fighters[match.SideP2].Health; h != 970 {
		t.Fatalf("P2 health = %d, want 970 (the hit lands)", h)
	}
	if got := result.State.Fighters[match.SideP2].Facing; got != match.FacingRight {
		t.Errorf("P2 facing in the match state = %v, want right (p2facing = -1)", got)
	}
}

// Two guarded strikes in one tick read the defender's life from one place:
// after a 15 chip leaves 5 life, a second 10 chip cannot be guarded with
// guard.kill 1 (it would kill), so it lands as a hit and life floors at 0.
func TestStrike_TwoGuardedStrikesInOneTickNeverLeaveNegativeLife(t *testing.T) {
	p1 := match.FighterState{Side: match.SideP1, Facing: match.FacingRight, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: 30}, Facing: match.FacingLeft, Health: 20}
	state, err := match.NewMatchState(1, 0, p1, p2)
	if err != nil {
		t.Fatal(err)
	}
	var runtimes [2]FighterRuntime
	runtimes[match.SideP1].Context.FighterState = p1
	runtimes[match.SideP2].Context.FighterState = p2
	defender := &runtimes[match.SideP2].Context
	defender.Guarding, defender.StateType = true, 'S'
	var sounds [2][]evaluator.SoundEvent
	cfg := TickConfig{Tick: 1, ComboWindow: 60}
	hit := func(damage string) {
		hitDef := cns.Controller{Type: "HitDef", Parameters: map[string]string{"damage": damage, "guardflag": "MA", "attr": "S, NA"}}
		strike(state, &runtimes, match.SideP1, hitDef, &runtimes[match.SideP1].Context, cfg, &sounds)
	}
	hit("30,15")
	if got := state.Fighters[match.SideP2].Health; got != 5 {
		t.Fatalf("after the first guarded strike: life %d, want 5", got)
	}
	hit("30,10")
	if got := state.Fighters[match.SideP2].Health; got != 0 {
		t.Errorf("after the second strike: life %d, want 0 (not guarded, floored)", got)
	}
}
