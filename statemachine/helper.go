package statemachine

import (
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func init() {
	RegisterController("Helper", applyHelper)
	RegisterController("DestroySelf", applyDestroySelf)
}

// applyHelper is the Helper controller (Ikemen GO helper.Run,
// bytecode.go:6161-6300): it records a spawn the engine turns into a helper
// running state stateno (0 by default) at pos from postype (p1 by default),
// facing the spawner's way, or away with a negative facing.
// pausemovetime and supermovetime let it move during a pause.
// shortcut: ownpal, size.* and the
// Ikemen GO extensions (immortal, preserve, standby...) are not read.
func applyHelper(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	s := evaluator.HelperSpawn{PosType: "p1", Facing: 1, Name: strings.Trim(strings.TrimSpace(ctrl.Parameters["name"]), `"`)}
	ints := []struct {
		key string
		dst *int
	}{{"id", &s.ID}, {"stateno", &s.StateNo}, {"facing", &s.Facing},
		{"pausemovetime", &s.PauseMoveTime}, {"supermovetime", &s.SuperMoveTime}}
	for _, p := range ints {
		v, ok, err := evalOptional(ctrl, p.key, &ctx)
		if err != nil {
			return ctx, err
		}
		if ok {
			*p.dst = int(v)
		}
	}
	// keyctrl lists the numbers 1 to 3, each turning one on (bytecode.go
	// helper_keyctrl).
	if raw, ok := ctrl.Parameters["keyctrl"]; ok {
		for _, part := range evaluator.SplitList(raw) {
			v, err := evaluator.EvaluateMut(part, &ctx)
			if err != nil {
				return ctx, err
			}
			if m := v.Int(); m >= 1 && m <= len(s.KeyCtrl) {
				s.KeyCtrl[m-1] = true
			}
		}
	}
	if raw, ok := ctrl.Parameters["postype"]; ok {
		s.PosType = strings.ToLower(strings.TrimSpace(raw))
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
			s.Pos[i] = v.Float()
		}
	}
	ctx.NewHelpers = append(ctx.NewHelpers, s)
	return ctx, nil
}

// applyDestroySelf is DestroySelf (char.go:7125-7150): it removes a helper
// at the end of the tick and, like ChangeState, ends its state's run
// (ApplyController reports it); a player ignores it, as does a helper under
// AssertSpecial nodestroyself.
// shortcut: recursive, removeexplods and removetexts are not read.
func applyDestroySelf(_ cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	if ctx.IsHelper && !ctx.Asserted.Has("nodestroyself") {
		ctx.Destroyed = true
	}
	return ctx, nil
}
