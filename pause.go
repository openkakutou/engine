package engine

import (
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
	"github.com/openkakutou/stage"
)

// pausedFor reports a fighter frozen by the running pause (Ikemen GO
// pauseStatus, char.go:2791-2802 for projectiles, the same for fighters): a
// SuperPause freezes it without super move time, a Pause without pause move
// time.
func pausedFor(state *match.MatchState, superMoveTime, pauseMoveTime int) bool {
	if state.SuperTime > 0 {
		return superMoveTime <= 0
	}
	if state.PauseTime > 0 {
		return pauseMoveTime <= 0
	}
	return false
}

// tickOrFreeze is tickFighter for a fighter the pause leaves moving, its
// move time counting down; a frozen one stays as it was, showing its current
// frame.
func tickOrFreeze(state *match.MatchState, progs fighterPrograms, runtime FighterRuntime, fighter match.FighterState, opponent evaluator.Opponent, in input.TickInput, tick int, bounds *stage.StageBoundaries, screen [2]float64, gravity float64, round int, seed int32) (fighterTickOutput, error) {
	if pausedFor(state, runtime.SuperMoveTime, runtime.PauseMoveTime) {
		runtime.Context.FighterState = fighter
		runtime.Context.Events = nil
		runtime.Unsupported, runtime.SkippedCalls = runtime.Unsupported[:0], runtime.SkippedCalls[:0]
		frame := currentFrame(findAnimation(progs.anims(&runtime.Context), runtime.Context.Anim), runtime.Context.AnimElapsed)
		return fighterTickOutput{Runtime: runtime, Frame: frame, Frozen: true}, nil
	}
	out, err := tickFighter(progs, runtime, fighter, opponent, in, tick, bounds, screen, gravity, round, seed)
	switch {
	case state.SuperTime > 0 && out.Runtime.SuperMoveTime > 0:
		out.Runtime.SuperMoveTime--
	case state.PauseTime > 0 && out.Runtime.PauseMoveTime > 0:
		out.Runtime.PauseMoveTime--
	}
	return out, err
}

// unhittableAttr is every attack: a SuperPause's unhittable fighter is hit
// by none of them.
var unhittableAttr, _ = evaluator.ParseAttr("SCA, AA, AT, AP", false)

// startPause starts the Pause or SuperPause the fighter asked for this tick,
// in the next tick as Ikemen GO buffers it (setPauseTime, setSuperPauseTime,
// char.go:9384-9440): the pause time, the fighter's move time; for a
// SuperPause the darkening, the power it adds, its effect explod (shown
// through the pause) and its unhittable time.
func startPause(state *match.MatchState, rt *FighterRuntime) {
	r := rt.Context.Pause
	rt.Context.Pause = evaluator.PauseRequest{}
	if !r.Set {
		return
	}
	if !r.Super {
		state.PauseTime, rt.PauseMoveTime = r.Time, r.MoveTime
		return
	}
	state.SuperTime, state.SuperDarken, rt.SuperMoveTime = r.Time, r.Darken, r.MoveTime
	c := &rt.Context
	c.Power = min(max(c.Power+r.PowerAdd, 0), c.PowerMax())
	state.Fighters[c.Side].Power = c.Power
	if r.Anim >= 0 && len(rt.Explods) < explodMax {
		rt.Explods = append(rt.Explods, Explod{Time: 1, Anim: r.Anim, Common: r.AnimCommon, Facing: c.Facing, Scale: [2]float64{1, 1},
			Position: explodAt(c.Position, c.Facing, r.Pos), RemoveTime: -2, OnTop: true, MovesInPause: true, PosType: "p1"})
	}
	if r.Unhittable {
		c.HitBy[len(c.HitBy)-1] = evaluator.HitBySlot{Attr: unhittableAttr, Time: r.Time + 1, Not: true}
	}
}
