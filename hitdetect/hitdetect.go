// Package hitdetect resolves Clsn (collision box) overlap between two
// fighters each simulation tick: hit boxes (Clsn1, attack) against hurt
// boxes (Clsn2, vulnerable), sourced from each fighter's currently active
// animation frame -- already fully resolved per-frame data exposed by
// character/air (Frame.Clsn1/Clsn2), regardless of whether that frame
// progression is being driven by CNS state-machine execution or .zss
// script execution. This package does not apply damage itself -- it only
// detects and reports collisions as HitEvents for a later package (damage/
// combo) to consume.
//
// A Clsn box's local coordinates increase downward, like screen/sprite
// space (confirmed against a real .air fixture: a standing character's
// Clsn2 box has a more-negative Top near the head and a Bottom of 0 at the
// feet), the opposite of match.Position.Y's "upward is positive"
// convention -- see .vibe/decisions/009 for the coordinate transform this
// implies, and for why this package holds to a zero-allocation result on
// the common zero-hit tick.
package hitdetect

import (
	"github.com/openkakutou/character/air"
	"github.com/openkakutou/engine/match"
)

// HitEvent reports one detected overlap between an attacker's Clsn1 box
// and a defender's Clsn2 box. AttackerBox/DefenderBox are the original,
// untransformed local air.ClsnBox values as found on each fighter's
// current frame -- not the world-space boxes actually compared -- so a
// consumer can trace an event back to the source .air data; recomputing
// world space from a fighter's live Position/Facing plus these values is
// cheap and lossless.
type HitEvent struct {
	Attacker    match.Side
	Defender    match.Side
	AttackerBox air.ClsnBox
	DefenderBox air.ClsnBox
}

// Detect checks both attack directions between state's two fighters --
// SideP1's Clsn1 boxes against SideP2's Clsn2 boxes, and SideP2's against
// SideP1's -- using each fighter's currently active animation frame, and
// returns one HitEvent per overlapping box pair found. frames must be
// indexed by Side the same way state.Fighters is: frames[match.SideP1] is
// P1's current frame, frames[match.SideP2] is P2's.
//
// A frame with no Clsn1 or Clsn2 boxes at all (e.g. an idle frame) simply
// contributes no events for that side -- there is no error path here, only
// geometry over whatever boxes each frame actually has.
func Detect(state *match.MatchState, frames [2]air.Frame) []HitEvent {
	var events []HitEvent
	events = detectDirection(events, state, frames, match.SideP1, match.SideP2)
	events = detectDirection(events, state, frames, match.SideP2, match.SideP1)
	return events
}

// detectDirection appends to events every HitEvent found between
// attacker's Clsn1 boxes and defender's Clsn2 boxes, and returns the
// (possibly still-nil) result -- so a tick with no overlaps anywhere never
// allocates.
func detectDirection(events []HitEvent, state *match.MatchState, frames [2]air.Frame, attacker, defender match.Side) []HitEvent {
	attackerState := state.Fighter(attacker)
	defenderState := state.Fighter(defender)

	for _, hitBox := range frames[attacker].Clsn1 {
		aMinX, aMinY, aMaxX, aMaxY := worldBox(attackerState.Position, attackerState.Facing, hitBox)

		for _, hurtBox := range frames[defender].Clsn2 {
			dMinX, dMinY, dMaxX, dMaxY := worldBox(defenderState.Position, defenderState.Facing, hurtBox)

			if overlaps(aMinX, aMinY, aMaxX, aMaxY, dMinX, dMinY, dMaxX, dMaxY) {
				events = append(events, HitEvent{
					Attacker:    attacker,
					Defender:    defender,
					AttackerBox: hitBox,
					DefenderBox: hurtBox,
				})
			}
		}
	}

	return events
}

// worldBox transforms box from a fighter's local .air coordinates (Left/
// Right along facing-relative X, Top/Bottom increasing downward) into a
// world-space axis-aligned bounding box: minX/maxX/minY/maxY, in that
// order. facing mirrors the X axis (FacingLeft negates and swaps Left/
// Right); the Y axis is always flipped (worldY = position.Y - localY),
// since match.Position.Y increases upward while a Clsn box's local Y
// increases downward. Both axes are returned already sorted (min <= max)
// regardless of a box's own Left/Top vs. Right/Bottom ordering.
func worldBox(position match.Position, facing match.Facing, box air.ClsnBox) (minX, minY, maxX, maxY float64) {
	left, right := float64(box.Left), float64(box.Right)
	if facing == match.FacingLeft {
		left, right = -right, -left
	}
	minX, maxX = position.X+left, position.X+right
	if minX > maxX {
		minX, maxX = maxX, minX
	}

	minY, maxY = position.Y-float64(box.Bottom), position.Y-float64(box.Top)
	if minY > maxY {
		minY, maxY = maxY, minY
	}

	return minX, minY, maxX, maxY
}

// overlaps reports whether two axis-aligned bounding boxes, each given as
// (minX, minY, maxX, maxY), intersect. Boxes that only touch at an edge
// (e.g. aMaxX == bMinX) do not count as overlapping.
func overlaps(aMinX, aMinY, aMaxX, aMaxY, bMinX, bMinY, bMaxX, bMaxY float64) bool {
	return aMinX < bMaxX && bMinX < aMaxX && aMinY < bMaxY && bMinY < aMaxY
}
