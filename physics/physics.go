// Package physics implements Ikemen GO's per-tick position update for a
// fighter (char.go posUpdate, 10102-10139): velocity integrates into
// position, the horizontal position is clamped to the stage's boundaries,
// then gravity is added to the vertical velocity. Position.Y follows an
// "upward is positive" convention, 0 being ground level.
//
// Like Ikemen GO's posUpdate, Step knows no ground: landing is the caller's
// (Tick changes a falling physics A fighter to state 52, decision 029).
//
// Stage boundary data and the gravity rate are both read-only inputs
// supplied by the caller on every call, not stored or defaulted internally
// -- see .vibe/decisions/007 for why.
package physics

import (
	"fmt"

	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/stage"
)

// Step advances f by one simulation tick in Ikemen GO's order: velocity
// integrates into position, the horizontal position is clamped to bounds so a
// fighter can never move outside the stage, then gravity (0 when the caller's
// physics type adds none) is subtracted from the vertical velocity.
//
// bounds is the current stage's horizontal movement limits, read-only. A
// nil bounds, meaning no stage boundary data was supplied, and a bounds
// whose Left is not strictly less than its Right, meaning the data is not a
// usable range, both produce a descriptive error rather than a panic or a
// silently unclamped result.
func Step(f match.FighterState, bounds *stage.StageBoundaries, gravity float64) (match.FighterState, error) {
	if bounds == nil {
		return match.FighterState{}, fmt.Errorf("physics: Step requires stage boundary data, got nil")
	}
	if bounds.Left >= bounds.Right {
		return match.FighterState{}, fmt.Errorf("physics: Step received an invalid stage boundary (left %d, right %d)", bounds.Left, bounds.Right)
	}

	out := f
	out.Position.X += out.Velocity.X
	out.Position.Y += out.Velocity.Y
	out.Position.X = clamp(out.Position.X, float64(bounds.Left), float64(bounds.Right))
	out.Velocity.Y -= gravity

	return out, nil
}

// clamp restricts x to the inclusive range [min, max].
func clamp(x, min, max float64) float64 {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}
