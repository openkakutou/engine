package statemachine

import (
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func init() {
	RegisterController("ChangeAnim2", applyChangeAnim2)
	for _, t := range [...]string{"BindToTarget", "TargetBind", "TargetFacing", "TargetState", "TargetLifeAdd", "TargetPowerAdd", "TargetVelSet", "TargetVelAdd", "TargetDrop"} {
		RegisterController(t, applyTargetOp)
	}
}

// applyChangeAnim2 is ChangeAnim with the animation of the state's owner, the
// opponent in a custom state (bytecode.go changeAnim2): it shows its
// opponent's animation.
func applyChangeAnim2(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	ctx, err := applyChangeAnim(ctrl, ctx)
	if err != nil {
		return ctx, err
	}
	ctx.AnimFromOpponent = ctx.InOpponentStates
	return ctx, nil
}

// Binding points of BindToTarget (Ikemen GO HMF).
const (
	BindFoot = iota
	BindMid
	BindHead
)

// bindPoint is BindToTarget's third pos value: Head, Mid, else Foot.
func bindPoint(raw string) int {
	point := strings.ToLower(strings.TrimSpace(raw))
	switch {
	case strings.HasPrefix(point, "h"):
		return BindHead
	case strings.HasPrefix(point, "m"):
		return BindMid
	}
	return BindFoot
}

// applyTargetOp records a Target* controller for the engine to apply to the
// fighter's target (Ikemen GO targetBind.Run and siblings, bytecode.go):
// TargetBind's pos (x, y) and time (1), TargetFacing's and TargetState's
// value, TargetLifeAdd's value, kill (1) and absolute (0), TargetPowerAdd's
// value, TargetVelSet / TargetVelAdd's x and y; BindToTarget's pos (x, y and
// the target's Foot, Mid or Head) and time (1).
// shortcut: the id and index parameters are not read: the target is the
// opponent while it is one.
func applyTargetOp(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	op := evaluator.TargetOp{Kind: strings.ToLower(ctrl.Type), Time: 1, Kill: true}
	if raw, ok := ctrl.Parameters["pos"]; ok {
		for i, part := range evaluator.SplitList(raw) {
			if i == 2 {
				// BindToTarget's third pos value: Foot, Mid or Head.
				op.Value = bindPoint(part)
				break
			}
			if part == "" {
				break
			}
			v, err := evaluator.EvaluateMut(part, &ctx)
			if err != nil {
				return ctx, err
			}
			if i == 0 {
				op.X, op.HasX = v.Float(), true
			} else {
				op.Y, op.HasY = v.Float(), true
			}
		}
	}
	x, hasX, err := evalOptional(ctrl, "x", &ctx)
	if err != nil {
		return ctx, err
	}
	if hasX {
		op.X, op.HasX = x, true
	}
	y, hasY, err := evalOptional(ctrl, "y", &ctx)
	if err != nil {
		return ctx, err
	}
	if hasY {
		op.Y, op.HasY = y, true
	}
	ints := []struct {
		key string
		dst *int
	}{{"value", &op.Value}, {"time", &op.Time}}
	for _, p := range ints {
		v, ok, err := evalOptional(ctrl, p.key, &ctx)
		if err != nil {
			return ctx, err
		}
		if ok {
			*p.dst = int(v)
		}
	}
	bools := []struct {
		key string
		dst *bool
	}{{"kill", &op.Kill}, {"absolute", &op.Absolute}}
	for _, p := range bools {
		v, ok, err := evalOptional(ctrl, p.key, &ctx)
		if err != nil {
			return ctx, err
		}
		if ok {
			*p.dst = v != 0
		}
	}
	ctx.TargetOps = append(ctx.TargetOps, op)
	return ctx, nil
}
