package statemachine

import (
	"fmt"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

// The get-hit and fall controllers common1's hit, fall, lie-down and get-up
// states use, each grounded in Ikemen GO's bytecode.go Run method and the
// char.go method it calls (commit 54d4148a).

func init() {
	RegisterController("HitVelSet", applyHitVelSet)
	RegisterController("HitFallSet", applyHitFallSet)
	RegisterController("HitFallVel", applyHitFallVel)
	RegisterController("HitFallDamage", applyHitFallDamage)
	RegisterController("PosFreeze", applyPosFreeze)
	RegisterController("DefenceMulSet", applyDefenceMulSet)
	RegisterController("AssertSpecial", applyAssertSpecial)
	RegisterController("HitBy", func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
		return applyHitBy(ctrl, ctx, false)
	})
	RegisterController("NotHitBy", func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
		return applyHitBy(ctrl, ctx, true)
	})
}

// applyHitBy is HitBy (not false) or NotHitBy, Ikemen GO's hitBySub
// (compiler_functions.go) and hitBy.runSub (bytecode.go:5517-5566): value
// fills slot 0 and value2 slot 1 (ignored when value is given); attr fills
// the given slot, and without a slot nothing is set; time defaults to 1, a
// slot past the eighth is the first.
func applyHitBy(ctrl cns.Controller, ctx evaluator.Context, not bool) (evaluator.Context, error) {
	slot, raw := -1, ""
	switch {
	case ctrl.Parameters["value"] != "":
		slot, raw = 0, ctrl.Parameters["value"]
	case ctrl.Parameters["value2"] != "":
		slot, raw = 1, ctrl.Parameters["value2"]
	case ctrl.Parameters["attr"] != "":
		raw = ctrl.Parameters["attr"]
		s, hasSlot, err := evalOptional(ctrl, "slot", &ctx)
		if err != nil {
			return ctx, err
		}
		if hasSlot {
			slot = max(int(s), 0)
		}
	default:
		return ctx, fmt.Errorf("%s needs value, value2 or attr", ctrl.Type)
	}
	attr, err := evaluator.ParseAttr(raw, false)
	if err != nil {
		return ctx, fmt.Errorf("%s attr %q: %w", ctrl.Type, raw, err)
	}
	time, hasTime, err := evalOptional(ctrl, "time", &ctx)
	if err != nil {
		return ctx, err
	}
	if !hasTime {
		time = 1
	}
	stack, _, err := evalOptional(ctrl, "stack", &ctx)
	if err != nil {
		return ctx, err
	}
	if slot < 0 {
		return ctx, nil
	}
	if slot >= len(ctx.HitBy) {
		slot = 0
	}
	ctx.HitBy[slot] = evaluator.HitBySlot{Attr: attr, Time: int(time), Not: not, Stack: stack != 0}
	return ctx, nil
}

// applyHitVelSet gives the fighter its get-hit velocity on each axis whose
// parameter is true (hitVelSet.Run): x the knock-back x, in world space
// (ghv.xvel times the facing, moved again by the facing), y the get-hit y.
func applyHitVelSet(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	x, hasX, err := evalOptional(ctrl, "x", &ctx)
	if err != nil {
		return ctx, err
	}
	y, hasY, err := evalOptional(ctrl, "y", &ctx)
	if err != nil {
		return ctx, err
	}
	if hasX && x != 0 {
		ctx.Velocity.X = ctx.GetHit.Xvel
	}
	if hasY && y != 0 {
		ctx.Velocity.Y = worldY(ctx.GetHit.Yvel)
	}
	return ctx, nil
}

// applyHitFallSet is hitFallSet (bytecode.go hitFallSet.Run, char.go:
// 9591-9604): value -1 (the default) keeps the fall flag, else sets it;
// xvel and yvel replace the fall velocities.
func applyHitFallSet(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	value, hasValue, err := evalOptional(ctrl, "value", &ctx)
	if err != nil {
		return ctx, err
	}
	if hasValue && value >= 0 {
		ctx.GetHit.Fall = value != 0
	}
	xv, hasX, err := evalOptional(ctrl, "xvel", &ctx)
	if err != nil {
		return ctx, err
	}
	if hasX {
		ctx.GetHit.FallXvel, ctx.GetHit.HasFallXvel = xv, true
	}
	yv, hasY, err := evalOptional(ctrl, "yvel", &ctx)
	if err != nil {
		return ctx, err
	}
	if hasY {
		ctx.GetHit.FallYvel = yv
	}
	return ctx, nil
}

// applyHitFallVel is hitFallVel (char.go:9579-9589): in a hit state, the fall
// velocities become the fighter's, x relative to its facing and only when
// set.
func applyHitFallVel(_ cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	if ctx.MoveType != 'H' {
		return ctx, nil
	}
	if ctx.GetHit.HasFallXvel {
		ctx.Velocity.X = worldX(ctx.GetHit.FallXvel, ctx.Facing)
	}
	ctx.Velocity.Y = worldY(ctx.GetHit.FallYvel)
	return ctx, nil
}

// applyHitFallDamage is hitFallDamage (char.go:9572-9577): in a hit state the
// fall damage is taken once (lifeAdd with fall.kill), then cleared.
func applyHitFallDamage(_ cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	if ctx.MoveType != 'H' || ctx.Health <= 0 {
		return ctx, nil
	}
	health := ctx.Health - ctx.GetHit.FallDamage
	if !ctx.GetHit.FallKill && health <= 0 {
		health = 1
	}
	ctx.Health = clampHealth(health, &ctx)
	ctx.GetHit.FallDamage = 0
	return ctx, nil
}

// applyPosFreeze is posFreeze (bytecode.go posFreeze.Run): value, 1 by
// default, keeps the fighter's position this tick.
func applyPosFreeze(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	value, hasValue, err := evalOptional(ctrl, "value", &ctx)
	if err != nil {
		return ctx, err
	}
	if !hasValue || value != 0 {
		ctx.PosFreeze = true
	}
	return ctx, nil
}

// applyDefenceMulSet is defenceMulSet (bytecode.go:11851-11888): value (1),
// mulType 1 sets the defence multiplier to it, 0 to its inverse; onHit
// delays it to the hits taken. A MUGEN character defaults to onHit 1 and
// mulType 0, an Ikemen GO one to onHit 0 and mulType 1.
func applyDefenceMulSet(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	value, hasValue, err := evalOptional(ctrl, "value", &ctx)
	if err != nil {
		return ctx, err
	}
	if !hasValue {
		value = 1
	}
	onHit, mulType := !ctx.IkemenChar, 0.0
	if ctx.IkemenChar {
		mulType = 1
	}
	onHitValue, hasOnHit, err := evalOptional(ctrl, "onhit", &ctx)
	if err != nil {
		return ctx, err
	}
	if hasOnHit {
		onHit = onHitValue != 0
	}
	mulTypeValue, hasMulType, err := evalOptional(ctrl, "multype", &ctx)
	if err != nil {
		return ctx, err
	}
	if hasMulType {
		mulType = mulTypeValue
	}
	if mulType != 0 {
		ctx.CustomDefence = value
	} else if value != 0 {
		ctx.CustomDefence = 1 / value
	}
	ctx.DefenceMulOnHit = onHit
	return ctx, nil
}

// assertFlagParams are the AssertSpecial parameters naming a flag.
var assertFlagParams = [...]string{"flag", "flag2", "flag3"}

// applyAssertSpecial is assertSpecial.Run (bytecode.go:5604-5632): each flag
// parameter is asserted, or cleared with enabled 0.
// shortcut: Ikemen GO's noko parameter and the global flags' effects are not
// modelled; the flags are only recorded.
func applyAssertSpecial(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	enabled, hasEnabled, err := evalOptional(ctrl, "enabled", &ctx)
	if err != nil {
		return ctx, err
	}
	on := !hasEnabled || enabled != 0
	named := false
	for _, key := range assertFlagParams {
		name, ok := ctrl.Parameters[key]
		if !ok {
			continue
		}
		named = true
		if err := ctx.Asserted.Set(name, on); err != nil {
			return ctx, fmt.Errorf("AssertSpecial %s: %w", key, err)
		}
	}
	if !named {
		return ctx, fmt.Errorf("AssertSpecial flag not specified")
	}
	return ctx, nil
}
