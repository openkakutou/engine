package statemachine

import (
	"testing"

	"github.com/openkakutou/engine/v2/evaluator"
)

func TestAngleControllers(t *testing.T) {
	ctx := evaluator.Context{AngleDrawScale: [2]float64{1, 1}}
	run(t, &ctx, "AngleSet", map[string]string{"value": "30"})
	run(t, &ctx, "AngleAdd", map[string]string{"value": "15"})
	run(t, &ctx, "AngleMul", map[string]string{"value": "2"})
	if ctx.Angle != 90 || ctx.AngleDraw {
		t.Errorf("angle %v draw %v, want 90 not drawn yet", ctx.Angle, ctx.AngleDraw)
	}
	run(t, &ctx, "AngleDraw", map[string]string{"scale": "2, 0.5"})
	if !ctx.AngleDraw || ctx.Angle != 90 || ctx.AngleDrawScale != [2]float64{2, 0.5} {
		t.Errorf("draw %v angle %v scale %v", ctx.AngleDraw, ctx.Angle, ctx.AngleDrawScale)
	}
	run(t, &ctx, "AngleSet", nil)
	if ctx.Angle != 0 {
		t.Errorf("bare AngleSet: angle %v, want 0 as MUGEN", ctx.Angle)
	}
}
