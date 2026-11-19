package physics

import (
	"testing"

	"github.com/openkakutou/engine/v2/match"
	"github.com/openkakutou/stage"
)

const testGravity = 0.5

func TestStep_IntegratesVelocityIntoPosition_WithoutGravity(t *testing.T) {
	f := match.FighterState{
		Position: match.Position{X: 10, Y: 0},
		Velocity: match.Velocity{X: 2, Y: 0},
	}
	bounds := &stage.StageBoundaries{Left: -100, Right: 100}

	got, err := Step(f, bounds, 0)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}

	want := match.Position{X: 12, Y: 0}
	if got.Position != want || got.Velocity.Y != 0 {
		t.Errorf("Position, Velocity.Y = %+v, %v, want %+v, 0", got.Position, got.Velocity.Y, want)
	}
}

// Ikemen GO posUpdate (char.go:10102-10139): the position moves by the
// velocity first, then gravity changes the velocity: Y = 5 + 1 = 6, then
// Velocity.Y = 1 - 0.5 = 0.5.
func TestStep_MovesThenAppliesGravity(t *testing.T) {
	f := match.FighterState{
		Position: match.Position{X: 0, Y: 5},
		Velocity: match.Velocity{X: 0, Y: 1},
	}
	bounds := &stage.StageBoundaries{Left: -100, Right: 100}

	got, err := Step(f, bounds, testGravity)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}

	if got.Position.Y != 6 || got.Velocity.Y != 0.5 {
		t.Errorf("Position.Y, Velocity.Y = %v, %v, want 6, 0.5", got.Position.Y, got.Velocity.Y)
	}
}

// Ikemen GO's posUpdate has no ground: the landing is the caller's (Tick,
// state 52), so a fighter crossing Y 0 is left below it.
func TestStep_DoesNotStopAtTheGround(t *testing.T) {
	f := match.FighterState{
		Position: match.Position{X: 0, Y: 0.25},
		Velocity: match.Velocity{X: 0, Y: -0.5},
	}
	bounds := &stage.StageBoundaries{Left: -100, Right: 100}

	got, err := Step(f, bounds, testGravity)
	if err != nil {
		t.Fatalf("Step returned error: %v", err)
	}

	if got.Position.Y != -0.25 || got.Velocity.Y != -1 {
		t.Errorf("Position.Y, Velocity.Y = %v, %v, want -0.25, -1", got.Position.Y, got.Velocity.Y)
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
