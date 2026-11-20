package statemachine

import (
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
