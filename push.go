package engine

import (
	"math"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/stage"
)

// cornerEpsilon is Ikemen GO's tolerance for "could not move the whole way"
// (char.go:14792).
const cornerEpsilon = 0.0001

// pushFighters is Ikemen GO's CharList.pushDetection for the one pair of a
// one-on-one match (char.go:14551-14779, commit 54d4148a), run by Tick after
// both fighters moved and before hit detection (char.go:14954-14960): when
// both have playerpush, their size boxes overlap (on X by more than 0) and
// their Clsn2 boxes in frames overlap too, they are pushed apart along X by
// the size-box overlap. p1 is Ikemen GO's c (the lower player id), p2 its
// getter. Positions are clamped to bounds, the stage limits physics.Step
// clamps to (Ikemen GO xScreenBound).
//
// shortcut: no Z axis, no AssertSpecial sizepushonly (item 050), no
// Width/Height controllers and no crouch/down sizebox constants: a
// crouching or lying fighter uses the ground box, as Ikemen GO does by
// default (char.go:4456-4459). See .vibe/decisions/031.
func pushFighters(p1, p2 *evaluator.Context, frames [2]air.Frame, bounds *stage.StageBoundaries) {
	if !p1.PlayerPush || !p2.PlayerPush {
		return
	}
	// Enemies are skipped when either pushes only its own team (char.go:14575-14580).
	if p1.PushAffectTeam < 0 || p2.PushAffectTeam < 0 {
		return
	}
	l1, r1, b1, t1 := sizeBox(p1)
	l2, r2, b2, t2 := sizeBox(p2)
	overlapX := axisOverlap(l1, r1, l2, r2)
	if overlapX <= 0 || axisOverlap(b1, t1, b2, t2) < 0 {
		return
	}
	if !clsn2Touch(p1, p2, frames) {
		return
	}

	f1, f2 := pushFactors(p1, p2)
	dir := pushDirection(p1, p2)
	old1, old2 := p1.Position.X, p2.Position.X
	move1, move2 := overlapX*f1, overlapX*f2
	if p1.PushPriority >= p2.PushPriority {
		p2.Position.X -= dir * move2
	}
	if p1.PushPriority <= p2.PushPriority {
		p1.Position.X += dir * move1
	}
	left, right := float64(bounds.Left), float64(bounds.Right)
	p1.Position.X = clampX(p1.Position.X, left, right)
	p2.Position.X = clampX(p2.Position.X, left, right)

	// A cornered fighter: the other takes the whole overlap times its own
	// pushfactor (char.go:14787-14820).
	if p1.PushPriority != p2.PushPriority {
		return
	}
	moved1, moved2 := math.Abs(p1.Position.X-old1), math.Abs(p2.Position.X-old2)
	cornered1 := moved1+cornerEpsilon < move1
	cornered2 := moved2+cornerEpsilon < move2
	if cornered1 && !cornered2 {
		extend(p2, old2, overlapX*pushFactor(p2)-moved2, left, right)
	}
	if cornered2 && !cornered1 {
		extend(p1, old1, overlapX*pushFactor(p1)-moved1, left, right)
	}
}

// extend moves ctx a further extra in the direction it was already pushed
// from old, clamped to the stage.
func extend(ctx *evaluator.Context, old, extra, left, right float64) {
	if extra <= 0 {
		return
	}
	if ctx.Position.X > old {
		ctx.Position.X = clampX(ctx.Position.X+extra, left, right)
	}
	if ctx.Position.X < old {
		ctx.Position.X = clampX(ctx.Position.X-extra, left, right)
	}
}

// pushFactors is how much of the overlap each fighter takes (char.go:14676-
// 14692): none for the higher priority, its own pushfactor for the lower,
// otherwise the average pushfactor split by the other's share of the weight.
func pushFactors(p1, p2 *evaluator.Context) (float64, float64) {
	if p1.PushPriority > p2.PushPriority {
		return 0, pushFactor(p2)
	}
	if p1.PushPriority < p2.PushPriority {
		return pushFactor(p1), 0
	}
	average := (pushFactor(p1) + pushFactor(p2)) / 2
	w1, w2 := p1.Consts.Lookup("size.weight").Float(), p2.Consts.Lookup("size.weight").Float()
	total := w1 + w2
	return average * w2 / total, average * w1 / total
}

func pushFactor(ctx *evaluator.Context) float64 {
	return ctx.Consts.Lookup("size.pushfactor").Float()
}

// pushDirection is +1 when p1 goes right and p2 left, -1 otherwise: the
// side p1 is on, or on a tie Ikemen GO's tie-break (char.go:14697-14725).
func pushDirection(p1, p2 *evaluator.Context) float64 {
	dist := p1.Position.X - p2.Position.X
	if math.Abs(dist) >= cornerEpsilon {
		return math.Copysign(1, dist)
	}
	switch {
	case p1.PushPriority > p2.PushPriority:
		return sideOf(p1.Position.X)
	case p1.PushPriority < p2.PushPriority:
		return -sideOf(p2.Position.X)
	case p1.MoveType == 'H' && p2.MoveType != 'H':
		return -facingSign(p1.Facing)
	case p1.MoveType != 'H' && p2.MoveType == 'H':
		return facingSign(p2.Facing)
	case p1.MoveType == 'A' && p2.MoveType != 'A':
		return facingSign(p2.Facing)
	case p1.MoveType != 'A' && p2.MoveType == 'A':
		return -facingSign(p1.Facing)
	case p1.Position.Y > p2.Position.Y: // higher, Ikemen GO's Y points down
		return facingSign(p2.Facing)
	}
	return -facingSign(p1.Facing)
}

func sideOf(x float64) float64 {
	if x >= 0 {
		return 1
	}
	return -1
}

func facingSign(f match.Facing) float64 {
	if f == match.FacingLeft {
		return -1
	}
	return 1
}

// sizeBox is ctx's size box in world space, Y up: left, right, bottom, top.
// The air box for state type A, the ground box otherwise, both as tall as
// size.height (Ikemen GO baseSizeBox, sizeToBox, char.go:4444-4459,
// 10484-10522).
func sizeBox(ctx *evaluator.Context) (left, right, bottom, top float64) {
	back, front := "size.ground.back", "size.ground.front"
	if ctx.StateType == 'A' {
		back, front = "size.air.back", "size.air.front"
	}
	l := -ctx.Consts.Lookup(back).Float()
	r := ctx.Consts.Lookup(front).Float()
	if ctx.Facing == match.FacingLeft {
		l, r = -r, -l
	}
	height := ctx.Consts.Lookup("size.height").Float()
	return ordered(ctx.Position.X+l, ctx.Position.X+r, ctx.Position.Y, ctx.Position.Y+height)
}

// clsn2Touch reports whether any Clsn2 box of p1 overlaps one of p2's, by
// Ikemen GO's rule where touching counts (clsnCheck, char.go:10848-10945).
func clsn2Touch(p1, p2 *evaluator.Context, frames [2]air.Frame) bool {
	for _, a := range frames[match.SideP1].Clsn2 {
		al, ar, ab, at := clsnWorld(p1, a)
		for _, b := range frames[match.SideP2].Clsn2 {
			bl, br, bb, bt := clsnWorld(p2, b)
			if axisOverlap(al, ar, bl, br) >= 0 && axisOverlap(ab, at, bb, bt) >= 0 {
				return true
			}
		}
	}
	return false
}

// clsnWorld is box in world space, Y up (decision 009).
func clsnWorld(ctx *evaluator.Context, box air.ClsnBox) (left, right, bottom, top float64) {
	l, r := float64(box.Left), float64(box.Right)
	if ctx.Facing == match.FacingLeft {
		l, r = -r, -l
	}
	return ordered(ctx.Position.X+l, ctx.Position.X+r, ctx.Position.Y-float64(box.Bottom), ctx.Position.Y-float64(box.Top))
}

func ordered(l, r, b, t float64) (float64, float64, float64, float64) {
	return math.Min(l, r), math.Max(l, r), math.Min(b, t), math.Max(b, t)
}

// axisOverlap is Ikemen GO's RectIntersectAABB overlap on one axis
// (common.go:325-357): negative when apart, and for a zero-width side the
// half-widths minus the distance between centers.
func axisOverlap(min1, max1, min2, max2 float64) float64 {
	overlap := math.Min(max1, max2) - math.Max(min1, min2)
	if overlap == 0 && (min1 == max1 || min2 == max2) {
		return (max1-min1)/2 + (max2-min2)/2 - math.Abs((min1+max1)/2-(min2+max2)/2)
	}
	return overlap
}

func clampX(x, left, right float64) float64 {
	return math.Max(left, math.Min(right, x))
}
