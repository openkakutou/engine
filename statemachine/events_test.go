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
	if err := applyCtrl(t, &ctx, "EnvShake", map[string]string{"time": "5", "ampl": "-4, 2"}); err != nil {
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
	if ex.Type != "EnvShake" || ex.Param("ampl").Raw != "-4, 2" || !sameValues(ex.Param("ampl").Values, []float64{-4, 2}) {
		t.Errorf("EnvShake event = %+v, want ampl kept raw and evaluated", ex)
	}
	if ex.Param("missing").Raw != "" {
		t.Error("an absent parameter reads empty")
	}
}

func TestApplyController_EveryVisualTypeIsSupported(t *testing.T) {
	for _, typ := range []string{"PalFX", "RemapPal", "MakeDust", "GameMakeAnim", "ForceFeedback", "Explod", "EnvShake", "SprPriority", "Width", "FallEnvShake", "AfterImage", "AfterImageTime"} {
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

// AfterImage and AfterImageTime reach the game as events, a word parameter
// (trans = add) as its text.
func TestAfterImage_IsReportedWithItsParameters(t *testing.T) {
	ctx := evaluator.Context{}
	if err := applyCtrl(t, &ctx, "AfterImage", map[string]string{"time": "20", "palbright": "30,30,30", "trans": "add"}); err != nil {
		t.Fatal(err)
	}
	if err := applyCtrl(t, &ctx, "AfterImageTime", map[string]string{"time": "0"}); err != nil {
		t.Fatal(err)
	}
	if len(ctx.Events) != 2 {
		t.Fatalf("events = %+v", ctx.Events)
	}
	ai := ctx.Events[0]
	if ai.Type != "AfterImage" || !sameValues(ai.Param("time").Values, []float64{20}) || ai.Param("trans").Raw != "add" || len(ai.Param("palbright").Values) != 3 {
		t.Errorf("AfterImage event = %+v", ai)
	}
	if ctx.Events[1].Type != "AfterImageTime" {
		t.Errorf("second event = %+v", ctx.Events[1])
	}
}

// screenBound.Run: value and movecamera are false unless given, as MUGEN.
func TestScreenBound_DefaultsToFalse(t *testing.T) {
	ctx := evaluator.Context{ScreenBound: true, MoveCamera: [2]bool{true, true}}
	if err := applyCtrl(t, &ctx, "ScreenBound", nil); err != nil {
		t.Fatal(err)
	}
	if ctx.ScreenBound || ctx.MoveCamera != [2]bool{} {
		t.Errorf("bare ScreenBound: bound %v camera %v, want all false", ctx.ScreenBound, ctx.MoveCamera)
	}
	if err := applyCtrl(t, &ctx, "ScreenBound", map[string]string{"value": "1", "movecamera": "0,1"}); err != nil {
		t.Fatal(err)
	}
	if !ctx.ScreenBound || ctx.MoveCamera != [2]bool{false, true} {
		t.Errorf("value 1, movecamera 0,1: bound %v camera %v", ctx.ScreenBound, ctx.MoveCamera)
	}
}
