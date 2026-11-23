package evaluator

import (
	"fmt"

	"github.com/openkakutou/character/air"
)

// The animation triggers follow Ikemen GO's Animation (anim.go, commit
// 54d4148a) and the Char methods over it (char.go:5566-5602). The fighter's
// playback is the pair (Context.Anim, Context.AnimElapsed); Ikemen's curelem,
// curelemtime and curtime are derived from it on demand rather than stored.
//
// shortcut: a held (-1) or zero-length element before the last one is taken
// as holding forever, as Tick's frame lookup does; Ikemen GO's animSeek skips
// such elements and its "negative prelooptime" case (anim.go:796-806) resets
// the element. No real character animation has one before its last element.

// ElementAt returns the 0-based element of anim playing animTime ticks after
// it started, and the ticks spent in it so far. Past the last element the
// animation continues from anim.LoopStart. An element whose Time is zero or
// negative holds indefinitely (Ikemen GO's -1), which also keeps a malformed
// animation from looping forever. anim must have at least one frame.
func ElementAt(anim air.Animation, animTime int) (elem, elapsed int) {
	elapsed = animTime
	for {
		f := anim.Frames[elem]
		if f.Time <= 0 || elapsed < f.Time {
			return elem, elapsed
		}
		elapsed -= f.Time
		elem++
		if elem >= len(anim.Frames) {
			elem = anim.LoopStart
			if elem < 0 || elem >= len(anim.Frames) {
				elem = 0
			}
		}
	}
}

// totalTime is Ikemen GO's Animation.totaltime (anim.go:443-462): the sum of
// the elements' durations, and -1 for an animation ending in a held (-1)
// element, which never wraps. anim must have at least one frame.
func totalTime(anim air.Animation) int {
	if anim.Frames[len(anim.Frames)-1].Time == -1 {
		return -1
	}
	total := 0
	for _, f := range anim.Frames {
		total += f.Time
	}
	return total
}

// curTime is Ikemen GO's Animation.curtime after animTime ticks of Action
// (anim.go:960-964): it counts ticks, and once it reaches the total time it
// is held for that tick, then restarts from total-looptime+1. An animation
// ending in a -1 element never wraps (totaltime -1, anim.go:445).
func curTime(anim air.Animation, animTime int) int {
	total := totalTime(anim)
	if total == -1 {
		return animTime
	}
	loop := 0
	for i, f := range anim.Frames {
		if i >= anim.LoopStart {
			loop += f.Time
		}
	}
	if animTime <= total || loop <= 0 {
		return animTime
	}
	return total - loop + 1 + (animTime-total-1)%loop
}

// animTime is Char.animTime (char.go:5604) over Animation.AnimTime
// (anim.go:617): curtime minus totaltime, 0 for a fighter without animation.
func animTime(ctx *Context) int {
	anim, ok := currentAnimation(ctx)
	if !ok {
		return 0
	}
	return curTime(anim, ctx.AnimElapsed) - totalTime(anim)
}

// currentAnimation returns the fighter's current animation, read from the
// table it comes from (ShownAnims). ok is false when it has none: no table, or an
// animation the character lacks (Ikemen GO's nil c.anim) or one without
// frames.
func currentAnimation(ctx *Context) (anim air.Animation, ok bool) {
	anim, ok = ctx.ShownAnims()[ctx.Anim]
	return anim, ok && len(anim.Frames) > 0
}

// animElemTime is Char.animElemTime (char.go:5573) over Animation.AnimElemTime
// (anim.go:621): ticks since element elem (1-based) began, negative before.
// ok is false for an element the animation lacks, Ikemen GO's undefined.
func animElemTime(ctx *Context, elem int) (t int, ok bool) {
	anim, ok := currentAnimation(ctx)
	if !ok || elem < 1 || elem > len(anim.Frames) {
		return 0, false
	}
	t = curTime(anim, ctx.AnimElapsed)
	for _, f := range anim.Frames[:elem-1] {
		t -= max(0, f.Time)
	}
	return t, true
}

// animElemNo is Char.animElemNo (char.go:5566) over Animation.AnimElemNo
// (anim.go:636-679): the 1-based element playing offset ticks from now,
// looking back for a negative or zero offset and forward for a positive one.
// ok is false for an offset before the animation began.
func animElemNo(ctx *Context, offset int) (no int, ok bool) {
	anim, ok := currentAnimation(ctx)
	if !ok || offset < -curTime(anim, ctx.AnimElapsed) {
		return 0, false
	}
	frames := anim.Frames
	last := len(frames) - 1
	cur, elapsed := ElementAt(anim, ctx.AnimElapsed)
	i, oldt := cur, 0
	time := offset + elapsed
	if offset <= 0 {
		loop := false
		for {
			if time >= 0 {
				return i + 1, true
			}
			i--
			if i < 0 || cur >= anim.LoopStart && i < anim.LoopStart {
				if time == oldt {
					break
				}
				oldt = time
				loop = true
				i = last
			}
			time += max(0, frames[i].Time)
			if loop && i == last && frames[i].Time == -1 {
				return i + 1, true
			}
		}
		return len(frames), true
	}
	for {
		time -= max(0, frames[i].Time)
		if time < 0 || i == last && frames[i].Time == -1 {
			return i + 1, true
		}
		i++
		if i >= len(frames) {
			if time == oldt {
				break
			}
			oldt = time
			i = anim.LoopStart
		}
	}
	return len(frames), true
}

// evalAnimCall evaluates AnimElemTime(e), AnimElemNo(t), AnimExist(n) and
// SelfAnimExist(n). The out-of-range cases Ikemen GO leaves undefined read 0,
// as an undefined value does in a comparison (bytecode.go:1328-1333).
//
// AnimExist and SelfAnimExist differ in Ikemen GO by whose animation table
// they read: AnimExist the table of the player the current animation came
// from (c.animPN, char.go:5583-5595, changed by ChangeAnim2 and by a state
// owned by another player), SelfAnimExist the character's own
// (c.gi().animTable, char.go:5597-5602).
func evalAnimCall(n callNode, ctx *Context) (Value, bool, error) {
	if len(n.args) != 1 {
		return Value{}, true, fmt.Errorf("evaluator: %s() expects 1 argument, got %d", n.name, len(n.args))
	}
	a, err := evalNode(n.args[0], ctx)
	if err != nil {
		return Value{}, true, err
	}
	arg := int(a.Number)
	switch n.name {
	case "animelemtime":
		t, _ := animElemTime(ctx, arg)
		return intValue(float64(t)), true, nil
	case "animelemno":
		no, _ := animElemNo(ctx, arg)
		return intValue(float64(no)), true, nil
	case "animexist":
		_, exists := ctx.ShownAnims()[arg]
		return boolValue(exists), true, nil
	case "selfanimexist":
		_, exists := ctx.Anims[arg]
		return boolValue(exists), true, nil
	}
	return Value{}, false, nil
}

// evalAnimElem is "AnimElem = n" and "AnimElem = n, op t": Ikemen GO compiles
// it to AnimElemTime(n) op t, with "= 0" when no comparison follows, and skips
// the comparison, leaving false, when the element does not exist
// (compiler.go:4297-4318).
func evalAnimElem(n animElemNode, ctx *Context) (Value, error) {
	t, ok := animElemTime(ctx, n.elem)
	if !ok {
		return boolValue(false), nil
	}
	if n.interval {
		in := inInterval(intValue(float64(t)), intValue(float64(n.lo)), intValue(float64(n.hi)), n.loOpen, n.hiOpen)
		return boolValue(in != n.not), nil
	}
	right, err := evalNode(n.value, ctx)
	if err != nil {
		return Value{}, err
	}
	v, _, err := evalComparisonOrLogical(n.op, intValue(float64(t)), right)
	return v, err
}
