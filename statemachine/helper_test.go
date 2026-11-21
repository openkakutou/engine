package statemachine

import (
	"math"
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func TestHelper_RecordsItsSpawn(t *testing.T) {
	ctx := evaluator.Context{}
	ctrl := cns.Controller{Type: "Helper", Parameters: map[string]string{"id": "1000+5", "stateno": "1100", "pos": "30, -40", "facing": "-1", "name": `"Fireball"`, "postype": "P2", "keyctrl": "1, 3"}}
	if _, err := ApplyController(ctrl, &ctx, nil); err != nil {
		t.Fatal(err)
	}
	want := evaluator.HelperSpawn{ID: 1005, StateNo: 1100, PosType: "p2", Pos: [2]float64{30, -40}, Facing: -1, Name: "Fireball", KeyCtrl: [3]bool{true, false, true}}
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

func TestExplod_RecordsItsSpawn(t *testing.T) {
	ctx := evaluator.Context{}
	ctx.Vars[0] = 2
	ctrl := cns.Controller{Type: "Explod", Parameters: map[string]string{"anim": "F5000+var(0)", "id": "-3", "pos": "10, -20", "postype": "Back", "facing": "-1", "vfacing": "-1", "bindtime": "-1", "removetime": "30", "vel": "1.5", "ontop": "1", "removeongethit": "1", "sprpriority": "4"}}
	if _, err := ApplyController(ctrl, &ctx, nil); err != nil {
		t.Fatal(err)
	}
	want := evaluator.ExplodSpawn{ID: 0, Anim: 5002, Common: true, PosType: "back", Pos: [2]float64{10, -20}, Vel: [2]float64{1.5, 0}, Facing: -1, VFacing: -1, BindTime: -1, RemoveTime: 30, SprPriority: 4, OnTop: true, RemoveOnGetHit: true}
	if len(ctx.NewExplods) != 1 || ctx.NewExplods[0] != want {
		t.Errorf("spawns = %+v, want %+v", ctx.NewExplods, want)
	}
}

// bytecode.go:6788-6791: a MUGEN 1.1 character's explod postype defaults to
// none; an Ikemen GO one's stays p1.
func TestExplod_AMugen11CharacterDefaultsToNoPosType(t *testing.T) {
	ctx := evaluator.Context{MugenVersion: [2]int{1, 1}}
	if _, err := ApplyController(cns.Controller{Type: "Explod"}, &ctx, nil); err != nil {
		t.Fatal(err)
	}
	if got := ctx.NewExplods[0].PosType; got != "" {
		t.Errorf("postype = %q, want none", got)
	}
}

func TestRemoveExplod_DefaultsToEveryExplod(t *testing.T) {
	ctx := evaluator.Context{}
	if _, err := ApplyController(cns.Controller{Type: "RemoveExplod"}, &ctx, nil); err != nil {
		t.Fatal(err)
	}
	if len(ctx.RemovedExplods) != 1 || ctx.RemovedExplods[0].ID != -1 {
		t.Errorf("removals = %+v, want id -1", ctx.RemovedExplods)
	}
}

// compiler.go:7319-7330 and bytecode.go:5136-5143: persistent 0 runs a
// controller once per state entry, N every N ticks; a negative state ignores
// it.
func TestStep_PersistentControllers(t *testing.T) {
	add := func(persistent string) cns.Controller {
		return cns.Controller{Type: "VarAdd", Triggers: []string{"1"}, Parameters: map[string]string{"var(0)": "1", "persistent": persistent}}
	}
	run := func(number int, persistent string, ticks int) int {
		states := map[int]cns.StateDef{number: {Number: number, Controllers: []cns.Controller{add(persistent)}}}
		ctx := evaluator.Context{}
		for range ticks {
			r, err := StepState(ctx, number, states)
			if err != nil {
				t.Fatal(err)
			}
			ctx = r.Context
		}
		return ctx.Vars[0]
	}
	for _, c := range []struct {
		number     int
		persistent string
		want       int
	}{{0, "0", 1}, {0, "1", 6}, {0, "", 6}, {0, "3", 2}, {0, "200", 6}, {-2, "0", 6}, {0, "-5", 1}} {
		if got := run(c.number, c.persistent, 6); got != c.want {
			t.Errorf("state %d persistent %q: ran %d times in 6 ticks, want %d", c.number, c.persistent, got, c.want)
		}
	}
}

func TestPersistentValue(t *testing.T) {
	for raw, want := range map[string]int32{"": 1, "0": math.MaxInt32, "1": 1, "7": 7, "129": 1, "-1": math.MaxInt32, " 2 ": 2} {
		if got := persistentValue(raw); got != want {
			t.Errorf("persistentValue(%q) = %d, want %d", raw, got, want)
		}
	}
}

// bytecode.go:7172-7176: a MUGEN character's ModifyExplod changes pos, vel
// and facing only along with a postype; an Ikemen GO one's always.
func TestModifyExplod_AMugenCharacterNeedsAPosTypeForPos(t *testing.T) {
	params := map[string]string{"id": "5", "pos": "10, 20", "removetime": "40"}
	ctx := evaluator.Context{}
	if _, err := ApplyController(cns.Controller{Type: "ModifyExplod", Parameters: params}, &ctx, nil); err != nil {
		t.Fatal(err)
	}
	m := ctx.ModifiedExplods[0]
	if m.ID != 5 || m.SetPos || !m.SetRemoveTime || m.RemoveTime != 40 || m.Relative {
		t.Errorf("MUGEN modification = %+v, want id 5, removetime only", m)
	}
	ikemen := evaluator.Context{IkemenChar: true}
	if _, err := ApplyController(cns.Controller{Type: "ModifyExplod", Parameters: params}, &ikemen, nil); err != nil {
		t.Fatal(err)
	}
	if m := ikemen.ModifiedExplods[0]; !m.SetPos || m.Pos != [2]float64{10, 20} || !m.Relative {
		t.Errorf("Ikemen GO modification = %+v, want pos 10,20, relative times", m)
	}
}

func TestModifyExplod_DefaultsToEveryExplod(t *testing.T) {
	ctx := evaluator.Context{}
	if _, err := ApplyController(cns.Controller{Type: "ModifyExplod", Parameters: map[string]string{"postype": "p2", "vel": "3"}}, &ctx, nil); err != nil {
		t.Fatal(err)
	}
	m := ctx.ModifiedExplods[0]
	if m.ID != -1 || !m.SetPosType || m.PosType != "p2" || !m.SetVel || m.Vel[0] != 3 {
		t.Errorf("modification = %+v", m)
	}
}
