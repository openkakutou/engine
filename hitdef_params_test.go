package engine

import (
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func TestResolveHitDef_EvaluatesNumbersAndKeepsWords(t *testing.T) {
	ctx := &evaluator.Context{}
	ctx.Vars[1] = 4
	ctx.FVars[0] = 0.5
	hd := cns.Controller{Type: "HitDef", Parameters: map[string]string{
		"damage": "floor(90*fvar(0)), var(1)", "attr": "S, NA", "pausetime": "12, 12",
		"ground.velocity": "-var(1)", "priority": "4, Hit", "sparkno": "S8210",
	}}
	got := resolveHitDef(hd, ctx).Parameters
	want := map[string]string{"damage": "45,4", "attr": "S, NA", "pausetime": "12, 12", "ground.velocity": "-4", "priority": "4, Hit", "sparkno": "S8210"}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if hd.Parameters["damage"] != "floor(90*fvar(0)), var(1)" {
		t.Error("the character's own HitDef was modified")
	}
}

func TestResolveHitDef_AnUnreadableExpressionIsKeptAsWritten(t *testing.T) {
	hd := cns.Controller{Parameters: map[string]string{"damage": "nosuchtrigger, 5"}}
	if got := resolveHitDef(hd, &evaluator.Context{}).Parameters["damage"]; got != "nosuchtrigger,5" {
		t.Errorf("damage = %q, want the expression kept", got)
	}
}

func TestResolveHitDef_ALiteralHitDefAllocatesNothing(t *testing.T) {
	hd := cns.Controller{Parameters: map[string]string{"damage": "30, 5", "pausetime": "12,12", "attr": "S, NA", "ground.velocity": "-4.5"}}
	ctx := &evaluator.Context{}
	resolveHitDef(hd, ctx)
	if n := testing.AllocsPerRun(20, func() { resolveHitDef(hd, ctx) }); n != 0 {
		t.Errorf("allocs = %v, want 0", n)
	}
}
