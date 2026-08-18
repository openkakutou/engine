// Package physics implements per-tick physics for a fighter: velocity
// integration, gravity while airborne, ground/air state transitions, and
// clamping a fighter's horizontal position against the current stage's
// boundaries.
//
// Ground/air state is deliberately not a stored field on match.FighterState
// -- it is derived every call from Position.Y and Velocity.Y, following the
// same wrap-rather-than-reopen precedent evaluator.Context set for an
// earlier item (see .vibe/decisions/002 and this item's own .vibe/decisions
// entry). Position.Y follows an "upward is positive" convention: 0 is
// ground level, and a fighter is airborne whenever it is above the ground
// or still moving upward off of it -- a fighter resting or falling into the
// ground (Y <= 0 and Velocity.Y <= 0) is grounded, and gravity does not
// apply to it.
//
// Stage boundary data and the gravity rate are both read-only inputs
// supplied by the caller on every call, not stored or defaulted internally
// -- see this item's .vibe/decisions entry for why.
package physics

import (
	"fmt"

	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/stage"
)

// Step advances f by one simulation tick: gravity is applied to its
// vertical velocity while airborne, velocity integrates into position,
// landing on the ground plane zeroes vertical velocity, and the resulting
// horizontal position is clamped to bounds so a fighter can never move
// outside the stage -- even when a single tick's velocity would otherwise
// carry it past the edge.
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
	if !grounded(out) {
		out.Velocity.Y -= gravity
	}

	out.Position.X += out.Velocity.X
	out.Position.Y += out.Velocity.Y

	if out.Position.Y <= 0 {
		out.Position.Y = 0
		out.Velocity.Y = 0
	}

	out.Position.X = clamp(out.Position.X, float64(bounds.Left), float64(bounds.Right))

	return out, nil
}

// grounded reports whether f is resting on, or still falling toward, the
// ground plane: at or below it (Position.Y <= 0) and not moving upward
// (Velocity.Y <= 0). A fighter given an upward impulse while at Y == 0 --
// the moment it leaves the ground -- is airborne starting the same tick,
// not grounded.
func grounded(f match.FighterState) bool {
	return f.Position.Y <= 0 && f.Velocity.Y <= 0
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
