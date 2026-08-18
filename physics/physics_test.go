package physics

import (
	"testing"

	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/stage"
)

const testGravity = 0.5

func TestStep_IntegratesVelocityIntoPosition_WhileGrounded(t *testing.T) {
	f := match.FighterState{
		Position: match.Position{X: 10, Y: 0},
		Velocity: match.Velocity{X: 2, Y: 0},
	}
	bounds := &stage.StageBoundaries{Left: -100, Right: 100}

	got, err := Step(f, bounds, testGravity)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}

	want := match.Position{X: 12, Y: 0}
	if got.Position != want {
		t.Errorf("Position = %+v, want %+v", got.Position, want)
	}
	if got.Velocity.Y != 0 {
		t.Errorf("Velocity.Y = %v, want 0 (gravity must not apply while grounded)", got.Velocity.Y)
	}
}

func TestStep_AppliesGravity_WhileAirborne(t *testing.T) {
	f := match.FighterState{
		Position: match.Position{X: 0, Y: 5},
		Velocity: match.Velocity{X: 0, Y: 1},
	}
	bounds := &stage.StageBoundaries{Left: -100, Right: 100}

	got, err := Step(f, bounds, testGravity)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}

	// Gravity decrements vertical velocity before it integrates into
	// position: 1 - 0.5 = 0.5, then Y = 5 + 0.5 = 5.5.
	if got.Velocity.Y != 0.5 {
		t.Errorf("Velocity.Y = %v, want 0.5", got.Velocity.Y)
	}
	if got.Position.Y != 5.5 {
		t.Errorf("Position.Y = %v, want 5.5", got.Position.Y)
	}
}

func TestStep_FullJumpArc_LeavesGroundApexAndLands(t *testing.T) {
	f := match.FighterState{
		Position: match.Position{X: 0, Y: 0},
		Velocity: match.Velocity{X: 0, Y: 2},
	}
	bounds := &stage.StageBoundaries{Left: -100, Right: 100}

	sawAirborne := false
	var apex float64
	for i := 0; i < 20; i++ {
		next, err := Step(f, bounds, testGravity)
		if err != nil {
			t.Fatalf("Step returned error on tick %d: %v", i, err)
		}
		if next.Position.Y > 0 {
			sawAirborne = true
		}
		if next.Position.Y > apex {
			apex = next.Position.Y
		}
		f = next
	}

	if !sawAirborne {
		t.Fatal("fighter never left the ground during the jump")
	}
	if apex <= 0 {
		t.Fatalf("apex = %v, want > 0", apex)
	}
	if f.Position.Y != 0 {
		t.Errorf("final Position.Y = %v, want 0 (fighter should have landed)", f.Position.Y)
	}
	if f.Velocity.Y != 0 {
		t.Errorf("final Velocity.Y = %v, want 0 (landing must zero vertical velocity)", f.Velocity.Y)
	}
}

func TestStep_TransitionsToGrounded_OnTheLandingTick(t *testing.T) {
	// One tick away from crossing back to the ground plane.
	f := match.FighterState{
		Position: match.Position{X: 0, Y: 0.3},
		Velocity: match.Velocity{X: 0, Y: -0.5},
	}
	bounds := &stage.StageBoundaries{Left: -100, Right: 100}

	got, err := Step(f, bounds, testGravity)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}

	if got.Position.Y != 0 {
		t.Errorf("Position.Y = %v, want 0 (clamped to the ground plane)", got.Position.Y)
	}
	if got.Velocity.Y != 0 {
		t.Errorf("Velocity.Y = %v, want 0 (landing zeroes vertical velocity)", got.Velocity.Y)
	}
}

func TestStep_ClampsPosition_ToStageRightBoundary(t *testing.T) {
	f := match.FighterState{
		Position: match.Position{X: 95, Y: 0},
		Velocity: match.Velocity{X: 50, Y: 0},
	}
	bounds := &stage.StageBoundaries{Left: -100, Right: 100}

	got, err := Step(f, bounds, testGravity)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}

	if got.Position.X != 100 {
		t.Errorf("Position.X = %v, want 100 (clamped to the right boundary despite overshoot)", got.Position.X)
	}
}

func TestStep_ClampsPosition_ToStageLeftBoundary(t *testing.T) {
	f := match.FighterState{
		Position: match.Position{X: -95, Y: 0},
		Velocity: match.Velocity{X: -50, Y: 0},
	}
	bounds := &stage.StageBoundaries{Left: -100, Right: 100}

	got, err := Step(f, bounds, testGravity)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}

	if got.Position.X != -100 {
		t.Errorf("Position.X = %v, want -100 (clamped to the left boundary despite overshoot)", got.Position.X)
	}
}

func TestStep_ReturnsError_WhenBoundsIsNil(t *testing.T) {
	f := match.FighterState{Position: match.Position{X: 0, Y: 0}}

	_, err := Step(f, nil, testGravity)
	if err == nil {
		t.Fatal("Step returned no error for nil stage boundary data, want a descriptive error")
	}
}

func TestStep_ReturnsError_WhenBoundsIsInvalid(t *testing.T) {
	f := match.FighterState{Position: match.Position{X: 0, Y: 0}}
	bounds := &stage.StageBoundaries{Left: 100, Right: -100}

	_, err := Step(f, bounds, testGravity)
	if err == nil {
		t.Fatal("Step returned no error for a left >= right stage boundary, want a descriptive error")
	}
}
