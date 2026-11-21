package statemachine

import (
	"fmt"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func init() {
	RegisterController("HitOverride", applyHitOverride)
}

// applyHitOverride is HitOverride (bytecode.go:11047-11100): slot (0 to 7, 0
// past them), attr, stateno (-1), time (1; 0 and below -1 are 1), keepstate.
// shortcut: forceair, forceguard and guardflag are not read.
func applyHitOverride(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	o := evaluator.HitOverride{StateNo: -1, Time: 1}
	attr, err := evaluator.ParseAttr(ctrl.Parameters["attr"], false)
	if err != nil {
		return ctx, fmt.Errorf("HitOverride attr: %w", err)
	}
	o.Attr = attr
	slot := 0
	ints := []struct {
		key string
		dst *int
	}{{"slot", &slot}, {"stateno", &o.StateNo}, {"time", &o.Time}}
	for _, p := range ints {
		v, ok, err := evalOptional(ctrl, p.key, &ctx)
		if err != nil {
			return ctx, err
		}
		if ok {
			*p.dst = int(v)
		}
	}
	if slot < 0 || slot > 7 {
		slot = 0
	}
	if o.Time < -1 || o.Time == 0 {
		o.Time = 1
	}
	keep, hasKeep, err := evalOptional(ctrl, "keepstate", &ctx)
	if err != nil {
		return ctx, err
	}
	o.KeepState = hasKeep && keep != 0
	ctx.HitOverrides[slot] = o
	return ctx, nil
}
