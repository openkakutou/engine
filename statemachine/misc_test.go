package statemachine

import (
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func run(t *testing.T, ctx *evaluator.Context, typ string, params map[string]string) {
	t.Helper()
	if _, err := ApplyController(cns.Controller{Type: typ, Parameters: params}, ctx, nil); err != nil {
		t.Fatal(err)
	}
}

// varRandom.Run: var(v) gets RandI(0, 1000), or of the range given.
func TestVarRandom_DrawsWithinItsRange(t *testing.T) {
	ctx := evaluator.Context{RandSeed: 12345}
	for range 50 {
		run(t, &ctx, "VarRandom", map[string]string{"v": "3", "range": "10, 12"})
		if v := ctx.Vars[3]; v < 10 || v > 12 {
			t.Fatalf("var(3) = %d, want 10 to 12", v)
		}
	}
	run(t, &ctx, "VarRandom", map[string]string{"v": "4", "range": "5"})
	if v := ctx.Vars[4]; v < 0 || v > 5 {
		t.Errorf("one-value range: var(4) = %d, want 0 to 5", v)
	}
	run(t, &ctx, "VarRandom", map[string]string{"v": "999", "range": "7,7"})
	if v, _ := ctx.VarValue("var", 999); v.Int() != 7 {
		t.Errorf("var(999) = %v, want 7 in the overflow vars", v)
	}
	seed := ctx.RandSeed
	run(t, &ctx, "VarRandom", map[string]string{"v": "-1"})
	if ctx.RandSeed == seed {
		t.Error("a negative var still draws, as Ikemen GO evaluates RandI before varSet")
	}
}

func TestPowerSet_IsKeptWithinTheMaximum(t *testing.T) {
	ctx := evaluator.Context{}
	run(t, &ctx, "PowerSet", map[string]string{"value": "5000"})
	if ctx.Power != DefaultMaxPower {
		t.Errorf("power = %d, want %d", ctx.Power, DefaultMaxPower)
	}
	run(t, &ctx, "PowerSet", map[string]string{"value": "-3"})
	if ctx.Power != 0 {
		t.Errorf("power = %d, want 0", ctx.Power)
	}
}

func TestAttackMulSet_MoveHitReset_AttackDist(t *testing.T) {
	ctx := evaluator.Context{MoveContactTime: 4, ContactGuarded: true}
	run(t, &ctx, "AttackMulSet", map[string]string{"value": "1.5"})
	run(t, &ctx, "MoveHitReset", nil)
	run(t, &ctx, "AttackDist", map[string]string{"x": "120, -5"})
	if !ctx.HasAttackMul || ctx.AttackMul != 1.5 {
		t.Errorf("attack mul %v %v, want 1.5", ctx.HasAttackMul, ctx.AttackMul)
	}
	if ctx.MoveContactTime != 0 || ctx.ContactGuarded {
		t.Errorf("contact %d %v, want reset", ctx.MoveContactTime, ctx.ContactGuarded)
	}
	if ctx.HitDefGuardDist != [2]float64{120, 0} {
		t.Errorf("guard dist %v, want 120, 0", ctx.HitDefGuardDist)
	}
}

func TestHitAdd_AddsToTheHitCountAndTheTargetsCombo(t *testing.T) {
	ctx := evaluator.Context{HitCount: 2}
	run(t, &ctx, "HitAdd", map[string]string{"value": "3"})
	if ctx.HitCount != 5 || len(ctx.TargetOps) != 1 || ctx.TargetOps[0].Kind != "hitadd" || ctx.TargetOps[0].Value != 3 {
		t.Errorf("hit count %d, ops %+v", ctx.HitCount, ctx.TargetOps)
	}
	run(t, &ctx, "HitAdd", map[string]string{"value": "0"})
	if len(ctx.TargetOps) != 1 {
		t.Error("HitAdd 0 queued a target op")
	}
}
