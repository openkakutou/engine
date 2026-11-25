package statemachine

import (
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func init() {
	RegisterController("AngleDraw", applyAngleDraw)
	RegisterController("AngleSet", func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
		return applyAngle(ctrl, ctx, func(_, v float64) float64 { return v })
	})
	RegisterController("AngleAdd", func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
		return applyAngle(ctrl, ctx, func(cur, v float64) float64 { return cur + v })
	})
	RegisterController("AngleMul", func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
		return applyAngle(ctrl, ctx, func(cur, v float64) float64 { return cur * v })
	})
}

// applyAngle is AngleSet, AngleAdd and AngleMul (bytecode.go): value (0
// when not given, as MUGEN) set, added to or multiplying the angle.
// shortcut: the x and y angles of Ikemen GO are not read.
func applyAngle(ctrl cns.Controller, ctx evaluator.Context, op func(cur, v float64) float64) (evaluator.Context, error) {
	v, _, err := evalOptional(ctrl, "value", &ctx)
	if err != nil {
		return ctx, err
	}
	ctx.Angle = op(ctx.Angle, v)
	return ctx, nil
}

// applyAngleDraw is angleDraw.Run: the fighter is drawn rotated this tick,
// value setting the angle first, scale multiplying the drawing scale.
// shortcut: x, y and pivot are not read.
func applyAngleDraw(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	v, ok, err := evalOptional(ctrl, "value", &ctx)
	if err != nil {
		return ctx, err
	}
	if ok {
		ctx.Angle = v
	}
	if raw, ok := ctrl.Parameters["scale"]; ok {
		for i, part := range evaluator.SplitList(raw) {
			if i > 1 || part == "" {
				break
			}
			s, err := evalPart(ctrl, "scale", part, &ctx)
			if err != nil {
				return ctx, err
			}
			ctx.AngleDrawScale[i] *= s.Float()
		}
	}
	ctx.AngleDraw = true
	return ctx, nil
}
