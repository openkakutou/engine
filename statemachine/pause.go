package statemachine

import (
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func init() {
	RegisterController("Pause", func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
		return applyPause(ctrl, ctx, false)
	})
	RegisterController("SuperPause", func(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
		return applyPause(ctrl, ctx, true)
	})
}

// applyPause records a Pause or SuperPause (bytecode.go pause.Run and
// superPause.Run, 11160-11245): time (30 for a SuperPause), movetime (0);
// a SuperPause also darkens (darken, 1), adds poweradd, shows anim (100 of
// the common fightfx; a S prefix is the character's, -1 none) at pos and
// makes the fighter unhittable (unhittable, 1).
// p2defmul (Const super.targetdefencemul, 1.5) raises the defence of an
// opponent already being hit; a value not above 0 keeps the default.
// shortcut: endcmdbuftime, pausebg, brightness and sound are not read.
func applyPause(ctrl cns.Controller, ctx evaluator.Context, super bool) (evaluator.Context, error) {
	r := evaluator.PauseRequest{Set: true, Super: super, Darken: super, Unhittable: super, Anim: -1}
	if super {
		r.Time, r.Anim, r.AnimCommon = 30, 100, true
		r.P2DefMul = ctx.Consts.Lookup("super.targetdefencemul").Number
		v, ok, err := evalOptional(ctrl, "p2defmul", &ctx)
		if err != nil {
			return ctx, err
		}
		if ok && v > 0 {
			r.P2DefMul = v
		}
	}
	ints := []struct {
		key string
		dst *int
	}{{"time", &r.Time}, {"movetime", &r.MoveTime}, {"poweradd", &r.PowerAdd}}
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
	}{{"darken", &r.Darken}, {"unhittable", &r.Unhittable}}
	for _, p := range bools {
		v, ok, err := evalOptional(ctrl, p.key, &ctx)
		if err != nil {
			return ctx, err
		}
		if ok {
			*p.dst = v != 0
		}
	}
	if raw, ok := ctrl.Parameters["anim"]; ok && super {
		raw = strings.TrimSpace(raw)
		r.AnimCommon = true
		if len(raw) > 1 && (raw[0] == 'S' || raw[0] == 's') {
			r.AnimCommon, raw = false, raw[1:]
		}
		v, err := evalPart(ctrl, "anim", raw, &ctx)
		if err != nil {
			return ctx, err
		}
		r.Anim = v.Int()
	}
	if raw, ok := ctrl.Parameters["pos"]; ok {
		for i, part := range evaluator.SplitList(raw) {
			if i > 1 || part == "" {
				break
			}
			v, err := evalPart(ctrl, "pos", part, &ctx)
			if err != nil {
				return ctx, err
			}
			r.Pos[i] = v.Float()
		}
	}
	r.Time = max(r.Time, 0)
	r.MoveTime = min(max(r.MoveTime, 0), r.Time)
	ctx.Pause = r
	return ctx, nil
}
