package statemachine

import (
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func TestHelper_RecordsItsSpawn(t *testing.T) {
	ctx := evaluator.Context{}
	ctrl := cns.Controller{Type: "Helper", Parameters: map[string]string{"id": "1000+5", "stateno": "1100", "pos": "30, -40", "facing": "-1", "name": `"Fireball"`, "postype": "P2"}}
	if _, err := ApplyController(ctrl, &ctx, nil); err != nil {
		t.Fatal(err)
	}
	want := evaluator.HelperSpawn{ID: 1005, StateNo: 1100, PosType: "p2", Pos: [2]float64{30, -40}, Facing: -1, Name: "Fireball"}
	if len(ctx.NewHelpers) != 1 || ctx.NewHelpers[0] != want {
		t.Errorf("spawns = %+v, want %+v", ctx.NewHelpers, want)
	}
}

func TestHelper_Defaults(t *testing.T) {
	ctx := evaluator.Context{}
	if _, err := ApplyController(cns.Controller{Type: "Helper"}, &ctx, nil); err != nil {
		t.Fatal(err)
	}
	want := evaluator.HelperSpawn{PosType: "p1", Facing: 1}
	if len(ctx.NewHelpers) != 1 || ctx.NewHelpers[0] != want {
		t.Errorf("spawns = %+v, want %+v", ctx.NewHelpers, want)
	}
}

func TestHelper_ABadExpressionIsAnError(t *testing.T) {
	ctx := evaluator.Context{}
	if _, err := ApplyController(cns.Controller{Type: "Helper", Parameters: map[string]string{"pos": "nosuchtrigger, 0"}}, &ctx, nil); err == nil {
		t.Error("want an error for an unknown trigger in pos")
	}
}

func TestDestroySelf_EndsAHelpersRunAndIsIgnoredByAPlayer(t *testing.T) {
	helper := evaluator.Context{IsHelper: true}
	stopped, err := ApplyController(cns.Controller{Type: "DestroySelf"}, &helper, nil)
	if err != nil || !stopped || !helper.Destroyed {
		t.Errorf("helper: stopped %v, destroyed %v, err %v; want both true", stopped, helper.Destroyed, err)
	}
	player := evaluator.Context{}
	stopped, err = ApplyController(cns.Controller{Type: "DestroySelf"}, &player, nil)
	if err != nil || stopped || player.Destroyed {
		t.Errorf("player: stopped %v, destroyed %v, err %v; want neither", stopped, player.Destroyed, err)
	}
}

// Ryu (SVC) writes "value =" and "value2=" empty: Ikemen GO takes the
// parameter as given (compiler.go:6411-6422), an attr that nothing hits.
func TestHitBy_AnEmptyValueIsGiven(t *testing.T) {
	ctx := evaluator.Context{}
	ctrl := cns.Controller{Type: "HitBy", Parameters: map[string]string{"value": "", "value2": ""}}
	if _, err := ApplyController(ctrl, &ctx, nil); err != nil {
		t.Fatal(err)
	}
	if ctx.HitBy[0].Time != 1 || ctx.HitBy[0].Attr != 0 {
		t.Errorf("slot 0 = %+v, want an empty attr for 1 tick", ctx.HitBy[0])
	}
}
