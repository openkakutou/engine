package statemachine

import (
	"fmt"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func init() {
	RegisterController("ReversalDef", applyReversalDef)
}

// applyReversalDef arms a ReversalDef (bytecode.go reversalDef): reversal.attr
// names the attacks it reverses, an empty one disarms it; the rest are the
// HitDef parameters it lands with (pausetime, sparkno, hitsound, p1stateno,
// p2stateno, p1facing, p2facing...).
func applyReversalDef(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	attr, err := evaluator.ParseAttr(ctrl.Parameters["reversal.attr"], false)
	if err != nil {
		return ctx, fmt.Errorf("ReversalDef reversal.attr: %w", err)
	}
	ctx.ReversalAttr, ctx.Reversal = attr, ctrl.Parameters
	if attr == 0 {
		ctx.Reversal = nil
	}
	return ctx, nil
}
