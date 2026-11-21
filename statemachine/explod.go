package statemachine

import (
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func init() {
	RegisterController("Explod", applyExplod)
	RegisterController("RemoveExplod", applyRemoveExplod)
}

// applyExplod is the Explod controller (bytecode.go:6773-7074, defaults
// char.go:1885-1930): it records an explod for the engine to make. anim
// defaults to 0, F names a common fightfx animation; postype p1, bindtime 1,
// removetime -2 (until the animation ends), facing and vfacing 1. A MUGEN
// 1.1 character's postype defaults to none.
// shortcut: space, random, scale, trans, the interpolation, palfx,
// afterimage, angle and the Ikemen GO extensions are not read.
func applyExplod(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	e := evaluator.ExplodSpawn{PosType: "p1", Facing: 1, VFacing: 1, BindTime: 1, RemoveTime: -2}
	if ctx.MugenVersion == [2]int{1, 1} && !ctx.IkemenChar {
		e.PosType = ""
	}
	if raw, ok := ctrl.Parameters["anim"]; ok {
		raw = strings.TrimSpace(raw)
		if len(raw) > 1 && (raw[0] == 'F' || raw[0] == 'f') {
			e.Common, raw = true, raw[1:]
		}
		if len(raw) > 1 && (raw[0] == 'S' || raw[0] == 's') {
			raw = raw[1:]
		}
		v, err := evaluator.EvaluateMut(raw, &ctx)
		if err != nil {
			return ctx, err
		}
		e.Anim = v.Int()
	}
	ints := []struct {
		key string
		dst *int
	}{{"id", &e.ID}, {"bindtime", &e.BindTime}, {"removetime", &e.RemoveTime}, {"sprpriority", &e.SprPriority}}
	for _, p := range ints {
		v, ok, err := evalOptional(ctrl, p.key, &ctx)
		if err != nil {
			return ctx, err
		}
		if ok {
			*p.dst = int(v)
		}
	}
	e.ID = max(e.ID, 0)
	for _, key := range [...]string{"supermovetime", "pausemovetime"} {
		v, ok, err := evalOptional(ctrl, key, &ctx)
		if err != nil {
			return ctx, err
		}
		if ok && v != 0 {
			e.MovesInPause = true
		}
	}
	flags := []struct {
		key string
		dst *int
	}{{"facing", &e.Facing}, {"vfacing", &e.VFacing}}
	for _, p := range flags {
		v, ok, err := evalOptional(ctrl, p.key, &ctx)
		if err != nil {
			return ctx, err
		}
		if ok && v < 0 {
			*p.dst = -1
		}
	}
	bools := []struct {
		key string
		dst *bool
	}{{"ontop", &e.OnTop}, {"removeongethit", &e.RemoveOnGetHit}, {"supermove", &e.MovesInPause}}
	for _, p := range bools {
		v, ok, err := evalOptional(ctrl, p.key, &ctx)
		if err != nil {
			return ctx, err
		}
		if ok {
			*p.dst = v != 0
		}
	}
	if raw, ok := ctrl.Parameters["postype"]; ok {
		e.PosType = strings.ToLower(strings.TrimSpace(raw))
		if e.PosType == "none" {
			e.PosType = ""
		}
	}
	pairs := []struct {
		key string
		dst *[2]float64
	}{{"pos", &e.Pos}, {"vel", &e.Vel}, {"accel", &e.Accel}}
	for _, p := range pairs {
		raw, ok := ctrl.Parameters[p.key]
		// vel, else velocity (compiler_functions.go:894-906).
		if !ok && p.key == "vel" {
			raw, ok = ctrl.Parameters["velocity"]
		}
		if !ok {
			continue
		}
		for i, part := range evaluator.SplitList(raw) {
			if i > 1 || part == "" {
				break
			}
			v, err := evaluator.EvaluateMut(part, &ctx)
			if err != nil {
				return ctx, err
			}
			p.dst[i] = v.Float()
		}
	}
	ctx.NewExplods = append(ctx.NewExplods, e)
	return ctx, nil
}

// applyRemoveExplod is RemoveExplod (bytecode.go:12321-12340): it removes
// the fighter's explods with that id, all without one.
// shortcut: the index parameter is not read.
func applyRemoveExplod(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	id := -1
	v, ok, err := evalOptional(ctrl, "id", &ctx)
	if err != nil {
		return ctx, err
	}
	if ok {
		id = int(v)
	}
	ctx.RemovedExplods = append(ctx.RemovedExplods, evaluator.ExplodRemoval{ID: id})
	return ctx, nil
}
