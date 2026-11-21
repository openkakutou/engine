package statemachine

import (
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func init() {
	RegisterController("Projectile", applyProjectile)
}

// projectileKeys are the Projectile parameters that are not its HitDef's.
var projectileKeys = map[string]bool{
	"projid": true, "projanim": true, "projhitanim": true, "projremanim": true, "projcancelanim": true,
	"projremove": true, "projremovetime": true, "projshadow": true, "projmisstime": true, "projhits": true,
	"projpriority": true, "projsprpriority": true, "projedgebound": true, "projstagebound": true,
	"projheightbound": true, "projlayerno": true, "projscale": true, "velocity": true, "velmul": true,
	"remvelocity": true, "accel": true, "offset": true, "postype": true,
}

// applyProjectile is the Projectile controller (Ikemen GO projectileSub,
// compiler_functions.go:2404-2585, defaults char.go:2719-2760): it records a
// spawn the engine turns into a projectile. projhitanim and projremanim
// default to -1 and projhitanim; projremovetime to -1; velmul to 1.
// projhits and projpriority default to 1, projcancelanim to projremanim,
// projmisstime to 0, projremove to 1.
// shortcut: postype other than p1 and the bounds are not read.
func applyProjectile(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	s := evaluator.ProjectileSpawn{RemoveTime: -1, HitAnim: -1, VelMul: [2]float64{1, 1}, HitDef: map[string]string{}, Hits: 1, Priority: 1}
	ints := []struct {
		key string
		dst *int
	}{{"projid", &s.ID}, {"projanim", &s.Anim}, {"projhitanim", &s.HitAnim}, {"projremovetime", &s.RemoveTime},
		{"projhits", &s.Hits}, {"projpriority", &s.Priority}, {"projmisstime", &s.MissTime}}
	for _, p := range ints {
		v, ok, err := evalOptional(ctrl, p.key, &ctx)
		if err != nil {
			return ctx, err
		}
		if ok {
			*p.dst = int(v)
		}
	}
	remAnim, hasRemAnim, err := evalOptional(ctrl, "projremanim", &ctx)
	if err != nil {
		return ctx, err
	}
	if hasRemAnim {
		s.RemAnim = int(remAnim)
	}
	pairs := []struct {
		key string
		dst *[2]float64
	}{{"velocity", &s.Velocity}, {"accel", &s.Accel}, {"velmul", &s.VelMul}, {"remvelocity", &s.RemVel}, {"offset", &s.Offset}}
	for _, p := range pairs {
		raw, ok := ctrl.Parameters[p.key]
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
	for k, v := range ctrl.Parameters {
		if !projectileKeys[k] {
			s.HitDef[k] = v
		}
	}
	if !hasRemAnim {
		s.RemAnim = s.HitAnim // projhitanim unless given (bytecode.go:9021-9023)
	}
	remove, hasRemove, err := evalOptional(ctrl, "projremove", &ctx)
	if err != nil {
		return ctx, err
	}
	s.KeepsOnHit = hasRemove && remove == 0
	s.CancelAnim = s.RemAnim // projremanim unless given (bytecode.go:9025-9027)
	cancelAnim, hasCancelAnim, err := evalOptional(ctrl, "projcancelanim", &ctx)
	if err != nil {
		return ctx, err
	}
	if hasCancelAnim {
		s.CancelAnim = max(int(cancelAnim), -1)
	}
	ctx.NewProjectiles = append(ctx.NewProjectiles, s)
	return ctx, nil
}
