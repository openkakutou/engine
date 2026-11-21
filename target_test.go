package engine

import (
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/match"
	"github.com/openkakutou/engine/v2/statemachine"
)

// boundPair is an attacker at x 0 facing right with the defender, hit, as
// its target at x 50 facing left.
func boundPair() (attacker, defender evaluator.Context) {
	attacker.HasTarget, attacker.Facing = true, match.FacingRight
	defender.MoveType, defender.Facing = 'H', match.FacingLeft
	defender.Position = match.Position{X: 50, Y: 0}
	return attacker, defender
}

func runOps(t *testing.T, attacker, defender *evaluator.Context, ctrls ...cns.Controller) {
	t.Helper()
	for _, c := range ctrls {
		if _, err := statemachine.ApplyController(c, attacker, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := applyTargetOps(attacker, defender, FighterProgram{}); err != nil {
		t.Fatal(err)
	}
}

// char.go:8618-8650: BindToTarget moves the fighter to its target plus pos,
// x toward the target's facing, and binds the target for time.
func TestBindToTarget_MovesTheFighterToItsTarget(t *testing.T) {
	a, d := boundPair()
	runOps(t, &a, &d, cns.Controller{Type: "BindToTarget", Parameters: map[string]string{"pos": "20, -10", "time": "5"}})
	if a.Position.X != 30 || a.Position.Y != 10 {
		t.Errorf("fighter at %v, want 30,10 (50 - 20 toward the target's left facing; 10 up)", a.Position)
	}
	if d.GetHit.BindTime != 5 || d.Position.X != 50 {
		t.Errorf("target bound %d at %v, want 5 ticks where it stands", d.GetHit.BindTime, d.Position)
	}
}

// HMF_H adds the target's size.head.pos; y not given leaves the height.
func TestBindToTarget_FromTheHeadAndOnlyX(t *testing.T) {
	a, d := boundPair()
	a.Position.Y = 7
	runOps(t, &a, &d, cns.Controller{Type: "BindToTarget", Parameters: map[string]string{"pos": "0, 0, Head"}})
	// Default size.head.pos -5,-90: x 50 - (-5), y 90 up.
	if a.Position.X != 55 || a.Position.Y != 90 {
		t.Errorf("from the head: %v, want 55,90", a.Position)
	}
	a, d = boundPair()
	a.Position.Y = 7
	runOps(t, &a, &d, cns.Controller{Type: "BindToTarget", Parameters: map[string]string{"pos": "10"}})
	if a.Position.X != 40 || a.Position.Y != 7 {
		t.Errorf("x only: %v, want 40,7", a.Position)
	}
	if d.GetHit.BindTime != 1 {
		t.Errorf("default time %d, want 1", d.GetHit.BindTime)
	}
}

func TestBindToTarget_NoTargetDoesNothing(t *testing.T) {
	a, d := boundPair()
	a.HasTarget = false
	runOps(t, &a, &d, cns.Controller{Type: "BindToTarget", Parameters: map[string]string{"pos": "20, 0"}})
	if a.Position.X != 0 || d.GetHit.BindTime != 0 {
		t.Errorf("without a target: fighter %v, bind %d", a.Position, d.GetHit.BindTime)
	}
}

func TestTargetBind_PutsTheTargetBeforeTheFighter(t *testing.T) {
	a, d := boundPair()
	runOps(t, &a, &d, cns.Controller{Type: "TargetBind", Parameters: map[string]string{"pos": "30, -20", "time": "3"}})
	if d.Position.X != 30 || d.Position.Y != 20 || d.GetHit.BindTime != 3 {
		t.Errorf("target at %v bound %d, want 30,20 for 3", d.Position, d.GetHit.BindTime)
	}
}
