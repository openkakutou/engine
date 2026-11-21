package statemachine

import (
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func init() {
	RegisterController("BindToParent", func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
		return applyBindToParent(ctrl, ctx, false)
	})
	RegisterController("BindToRoot", func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
		return applyBindToParent(ctrl, ctx, true)
	})
}

// applyBindToParent is bindToParent.Run and bindToRoot (bytecode.go): time
// (1), facing (kept unless given) and pos (0,0) bind the helper to its
// parent or root; a player has none and does nothing.
func applyBindToParent(ctrl cns.Controller, ctx evaluator.Context, root bool) (evaluator.Context, error) {
	if !ctx.IsHelper {
		return ctx, nil
	}
	b := evaluator.HelperBind{Root: root, Time: 1, Facing: ctx.Bind.Facing}
	t, ok, err := evalOptional(ctrl, "time", &ctx)
	if err != nil {
		return ctx, err
	}
	if ok {
		b.Time = int(t)
	}
	f, ok, err := evalOptional(ctrl, "facing", &ctx)
	if err != nil {
		return ctx, err
	}
	if ok && f < 0 {
		b.Facing = -1
	}
	if ok && f > 0 {
		b.Facing = 1
	}
	if raw, ok := ctrl.Parameters["pos"]; ok {
		for i, part := range evaluator.SplitList(raw) {
			if i > 1 || part == "" {
				break
			}
			v, err := evaluator.EvaluateMut(part, &ctx)
			if err != nil {
				return ctx, err
			}
			b.Pos[i] = v.Float()
		}
	}
	ctx.Bind = b
	return ctx, nil
}
