package statemachine

import (
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func applyMap(t *testing.T, ctx *evaluator.Context, typ string, params map[string]string) {
	t.Helper()
	if _, err := ApplyController(cns.Controller{Type: typ, Parameters: params}, ctx, func(int) bool { return true }); err != nil {
		t.Fatal(err)
	}
}

func mapValue(t *testing.T, ctx evaluator.Context, name string) float64 {
	t.Helper()
	v, err := evaluator.Evaluate("map("+name+")", ctx)
	if err != nil {
		t.Fatal(err)
	}
	return v.Number
}

// char.go:9808-9852 mapSetValue: MapSet stores the value under the
// lower-cased name, MapAdd adds to it (0 when unset); kfm_zss AI.zss uses
// both (mapSet{map: "gurt"; value: 10}, mapAdd{map: "gurt"; value: -1}).
func TestApplyController_MapSetAndMapAdd(t *testing.T) {
	var ctx evaluator.Context
	applyMap(t, &ctx, "MapSet", map[string]string{"map": `"Gurt"`, "value": "10"})
	if got := mapValue(t, ctx, "gurt"); got != 10 {
		t.Errorf("after MapSet gurt = %v, want 10", got)
	}
	applyMap(t, &ctx, "mapAdd", map[string]string{"map": `"gurt"`, "value": "-1"})
	if got := mapValue(t, ctx, "gurt"); got != 9 {
		t.Errorf("after MapAdd gurt = %v, want 9", got)
	}
	applyMap(t, &ctx, "MapAdd", map[string]string{"map": "fresh", "value": "2.5"})
	if got := mapValue(t, ctx, "fresh"); got != 2.5 {
		t.Errorf("MapAdd on an unset name = %v, want 2.5", got)
	}
}

func TestApplyController_MapSetWithoutNameOrValueFails(t *testing.T) {
	var ctx evaluator.Context
	if _, err := ApplyController(cns.Controller{Type: "MapSet", Parameters: map[string]string{"value": "1"}}, &ctx, func(int) bool { return true }); err == nil {
		t.Error("MapSet without map: want an error")
	}
	if _, err := ApplyController(cns.Controller{Type: "MapSet", Parameters: map[string]string{"map": "x"}}, &ctx, func(int) bool { return true }); err == nil {
		t.Error("MapSet without value: want an error")
	}
}
