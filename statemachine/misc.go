package statemachine

import (
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func init() {
	RegisterController("VarRandom", applyVarRandom)
	RegisterController("PowerSet", applyPowerSet)
	RegisterController("AttackMulSet", applyAttackMulSet)
	RegisterController("MoveHitReset", applyMoveHitReset)
	RegisterController("HitAdd", applyHitAdd)
	RegisterController("AttackDist", applyAttackDist)
	RegisterController("Offset", applyOffset)
	RegisterController("ExplodBindTime", applyExplodBindTime)
}

// applyVarRandom is varRandom.Run (bytecode.go): var(v) gets a random
// integer of range (0 to 1000; one value is 0 to it).
func applyVarRandom(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	v, _, err := evalOptional(ctrl, "v", &ctx)
	if err != nil {
		return ctx, err
	}
	lo, hi := int32(0), int32(1000)
	if raw, ok := ctrl.Parameters["range"]; ok {
		var bounds []int32
		for _, part := range evaluator.SplitList(raw) {
			if part == "" || len(bounds) == 2 {
				break
			}
			b, err := evaluator.EvaluateMut(part, &ctx)
			if err != nil {
				return ctx, err
			}
			bounds = append(bounds, int32(b.Int()))
		}
		switch len(bounds) {
		case 1:
			hi = bounds[0]
		case 2:
			lo, hi = bounds[0], bounds[1]
		}
	}
	// RandI draws before varSet checks the index (bytecode.go varRandom.Run);
	// AssignVar sets nothing for a negative one.
	drawn := evaluator.RandomRange(&ctx, lo, hi)
	_, err = ctx.AssignVar("var", int(v), evaluator.IntValue(int(drawn)))
	return ctx, err
}

// applyPowerSet is powerSet.Run: the power is value, kept within 0 and the
// fighter's maximum.
func applyPowerSet(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	v, ok, err := evalOptional(ctrl, "value", &ctx)
	if err != nil || !ok {
		return ctx, err
	}
	ctx.Power = min(max(int(v), 0), ctx.PowerMax())
	return ctx, nil
}

// applyAttackMulSet is attackMulSet.Run: value (or damage) multiplies the
// damage the fighter deals.
// shortcut: the red life, dizzy and guard points multipliers are not read.
func applyAttackMulSet(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	for _, key := range [...]string{"value", "damage"} {
		v, ok, err := evalOptional(ctrl, key, &ctx)
		if err != nil {
			return ctx, err
		}
		if ok {
			ctx.AttackMul, ctx.HasAttackMul = v, true
		}
	}
	return ctx, nil
}

// applyMoveHitReset is moveHitReset.Run (clearMoveHit, char.go:4993): the
// move's contact is forgotten.
func applyMoveHitReset(_ cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	ctx.MoveContactTime, ctx.ContactGuarded = 0, false
	return ctx, nil
}

// applyHitAdd is hitAdd.Run (char.go:7814-7835): value adds to the
// fighter's hit count and to its target's combo.
func applyHitAdd(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	v, ok, err := evalOptional(ctrl, "value", &ctx)
	if err != nil || !ok || int(v) == 0 {
		return ctx, err
	}
	ctx.HitCount += int(v)
	ctx.TargetOps = append(ctx.TargetOps, evaluator.TargetOp{Kind: "hitadd", Value: int(v)})
	return ctx, nil
}

// applyAttackDist is attackDist.Run: x sets the current HitDef's guard.dist
// reach, front and back, never below 0.
// shortcut: y is not read.
func applyAttackDist(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	raw, ok := ctrl.Parameters["x"]
	if !ok {
		return ctx, nil
	}
	for i, part := range evaluator.SplitList(raw) {
		if i > 1 || part == "" {
			break
		}
		v, err := evaluator.EvaluateMut(part, &ctx)
		if err != nil {
			return ctx, err
		}
		ctx.HitDefGuardDist[i] = max(v.Float(), 0)
	}
	return ctx, nil
}

// applyOffset is offset.Run: x and y offset the fighter's drawing this tick.
func applyOffset(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	for i, key := range [...]string{"x", "y"} {
		v, ok, err := evalOptional(ctrl, key, &ctx)
		if err != nil {
			return ctx, err
		}
		if ok {
			ctx.DrawOffset[i] = v
		}
	}
	return ctx, nil
}

// applyExplodBindTime is explodBindTime.Run: the fighter's explods with id
// (every one for -1) take bindtime time (0), a ModifyExplod of it alone.
func applyExplodBindTime(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	m := evaluator.ExplodModification{ID: -1, SetBindTime: true}
	id, ok, err := evalOptional(ctrl, "id", &ctx)
	if err != nil {
		return ctx, err
	}
	if ok {
		m.ID = int(id)
	}
	t, _, err := evalOptional(ctrl, "time", &ctx)
	if err != nil {
		return ctx, err
	}
	m.BindTime = int(t)
	ctx.ModifiedExplods = append(ctx.ModifiedExplods, m)
	return ctx, nil
}
