package statemachine

import (
	"testing"

	"github.com/openkakutou/engine/v2/evaluator"
)

// Visual and feedback controllers are reported, with their parameters
// evaluated, for the game to draw (item 052); the engine draws nothing.
func TestApplyController_VisualControllersAreReportedAsEvents(t *testing.T) {
	var ctx evaluator.Context
	ctx.Vars[0] = 7
	if err := applyCtrl(t, &ctx, "PalFX", map[string]string{"time": "10", "add": "var(0)*10, 0, -5"}); err != nil {
		t.Fatal(err)
	}
	if err := applyCtrl(t, &ctx, "Explod", map[string]string{"anim": "F5000", "pos": "10, -20"}); err != nil {
		t.Fatal(err)
	}
	if len(ctx.Events) != 2 {
		t.Fatalf("events = %+v, want 2", ctx.Events)
	}
	pal := ctx.Events[0]
	if pal.Type != "PalFX" || !sameValues(pal.Param("add").Values, []float64{70, 0, -5}) || !sameValues(pal.Param("time").Values, []float64{10}) {
		t.Errorf("PalFX event = %+v", pal)
	}
	ex := ctx.Events[1]
	if ex.Type != "Explod" || ex.Param("anim").Raw != "F5000" || !sameValues(ex.Param("pos").Values, []float64{10, -20}) {
		t.Errorf("Explod event = %+v, want anim kept raw (F prefix) and pos evaluated", ex)
	}
	if ex.Param("missing").Raw != "" {
		t.Error("an absent parameter reads empty")
	}
}

func TestApplyController_EveryVisualTypeIsSupported(t *testing.T) {
	for _, typ := range []string{"PalFX", "RemapPal", "MakeDust", "GameMakeAnim", "ForceFeedback", "Explod", "EnvShake", "SprPriority", "Width", "FallEnvShake"} {
		if !IsSupportedController(typ) {
			t.Errorf("%s is not supported", typ)
		}
	}
}

// bytecode.go sprPriority.Run sets the draw priority; width.Run the edge
// widths for this tick (front, back).
func TestApplyController_SprPriorityAndWidth(t *testing.T) {
	var ctx evaluator.Context
	if err := applyCtrl(t, &ctx, "SprPriority", map[string]string{"value": "2"}); err != nil {
		t.Fatal(err)
	}
	if ctx.SprPriority != 2 {
		t.Errorf("SprPriority = %d, want 2", ctx.SprPriority)
	}
	if err := applyCtrl(t, &ctx, "Width", map[string]string{"edge": "20, 5"}); err != nil {
		t.Fatal(err)
	}
	if ctx.WidthEdge != [2]float64{20, 5} {
		t.Errorf("WidthEdge = %v, want 20 5", ctx.WidthEdge)
	}
}

// char.go fallEnvShake: the HitDef's fall.envshake is reported once, then
// spent.
func TestApplyController_FallEnvShakeReportsTheHitsShakeOnce(t *testing.T) {
	ctx := hitCtx()
	ctx.GetHit.FallEnvShake = evaluator.EnvShake{Time: 12, Freq: 60, Ampl: -4, Phase: 0}
	for range 2 {
		if err := applyCtrl(t, &ctx, "FallEnvShake", nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(ctx.Events) != 1 || ctx.Events[0].Type != "EnvShake" || !sameValues(ctx.Events[0].Param("time").Values, []float64{12}) || !sameValues(ctx.Events[0].Param("ampl").Values, []float64{-4}) {
		t.Errorf("events = %+v, want one 12-tick EnvShake", ctx.Events)
	}
}

func sameValues(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
