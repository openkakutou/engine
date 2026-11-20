package statemachine

import (
	"github.com/openkakutou/character/cns"
	"testing"

	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/match"
)

// hitCtx is a fighter facing left in a hit state, knocked with world x
// velocity 4 (to the right) and y velocity -6 (up, content units).
func hitCtx() evaluator.Context {
	var ctx evaluator.Context
	ctx.MoveType = 'H'
	ctx.Facing = match.FacingLeft
	ctx.Health = 500
	ctx.GetHit = evaluator.GetHit{Active: true, Xvel: 4, Yvel: -6, FallYvel: -4.5, FallDamage: 70, FallKill: true}
	return ctx
}

// bytecode.go hitVelSet.Run: x and y, when true, take the get-hit
// velocity (vel[0] = ghv.xvel * facing, a world displacement of ghv.xvel).
func TestApplyController_HitVelSet(t *testing.T) {
	ctx := hitCtx()
	if err := applyCtrl(t, &ctx, "HitVelSet", map[string]string{"x": "1", "y": "1"}); err != nil {
		t.Fatal(err)
	}
	if ctx.Velocity.X != 4 || ctx.Velocity.Y != 6 {
		t.Errorf("velocity = %v, want x 4 (world) and y 6 (up)", ctx.Velocity)
	}
	ctx = hitCtx()
	ctx.Velocity = match.Velocity{X: 1, Y: 1}
	if err := applyCtrl(t, &ctx, "HitVelSet", map[string]string{"x": "0", "y": "1"}); err != nil {
		t.Fatal(err)
	}
	if ctx.Velocity.X != 1 || ctx.Velocity.Y != 6 {
		t.Errorf("x 0: velocity = %v, want x kept 1, y 6", ctx.Velocity)
	}
}

// char.go:9591-9604 hitFallSet: value -1 keeps the fall flag, 0 or 1 sets
// it; xvel and yvel set the fall velocities.
func TestApplyController_HitFallSetAndHitFallVel(t *testing.T) {
	ctx := hitCtx()
	if err := applyCtrl(t, &ctx, "HitFallSet", map[string]string{"value": "1", "xvel": "-2", "yvel": "-3"}); err != nil {
		t.Fatal(err)
	}
	if !ctx.GetHit.Fall || !ctx.GetHit.HasFallXvel || ctx.GetHit.FallXvel != -2 || ctx.GetHit.FallYvel != -3 {
		t.Errorf("get-hit = %+v, want fall with velocities -2, -3", ctx.GetHit)
	}
	if err := applyCtrl(t, &ctx, "HitFallSet", map[string]string{"value": "-1"}); err != nil {
		t.Fatal(err)
	}
	if !ctx.GetHit.Fall {
		t.Error("value -1 must keep the fall flag")
	}
	// char.go:9579-9589 hitFallVel: x is relative to the facing (left here,
	// so -2 forward is +2 world), y is set.
	if err := applyCtrl(t, &ctx, "HitFallVel", nil); err != nil {
		t.Fatal(err)
	}
	if ctx.Velocity.X != 2 || ctx.Velocity.Y != 3 {
		t.Errorf("after HitFallVel velocity = %v, want 2, 3", ctx.Velocity)
	}
}

func TestApplyController_HitFallVelWithoutFallXVelocityKeepsX(t *testing.T) {
	ctx := hitCtx()
	ctx.Velocity.X = 5
	if err := applyCtrl(t, &ctx, "HitFallVel", nil); err != nil {
		t.Fatal(err)
	}
	if ctx.Velocity.X != 5 || ctx.Velocity.Y != 4.5 {
		t.Errorf("velocity = %v, want x kept 5, y 4.5", ctx.Velocity)
	}
	ctx.MoveType = 'I'
	ctx.Velocity = match.Velocity{}
	if err := applyCtrl(t, &ctx, "HitFallVel", nil); err != nil {
		t.Fatal(err)
	}
	if ctx.Velocity != (match.Velocity{}) {
		t.Error("not in a hit state: HitFallVel does nothing")
	}
}

// char.go:9572-9577 hitFallDamage: the fall damage once, then 0; fall.kill
// 0 keeps the fighter at 1 life.
func TestApplyController_HitFallDamage(t *testing.T) {
	ctx := hitCtx()
	if err := applyCtrl(t, &ctx, "HitFallDamage", nil); err != nil {
		t.Fatal(err)
	}
	if err := applyCtrl(t, &ctx, "HitFallDamage", nil); err != nil {
		t.Fatal(err)
	}
	if ctx.Health != 430 || ctx.GetHit.FallDamage != 0 {
		t.Errorf("health %d, fall damage left %d; want 430 and 0", ctx.Health, ctx.GetHit.FallDamage)
	}
	ctx = hitCtx()
	ctx.Health, ctx.GetHit.FallKill = 50, false
	if err := applyCtrl(t, &ctx, "HitFallDamage", nil); err != nil {
		t.Fatal(err)
	}
	if ctx.Health != 1 {
		t.Errorf("fall.kill 0: health %d, want 1", ctx.Health)
	}
}

func TestApplyController_PosFreeze(t *testing.T) {
	var ctx evaluator.Context
	if err := applyCtrl(t, &ctx, "PosFreeze", nil); err != nil {
		t.Fatal(err)
	}
	if !ctx.PosFreeze {
		t.Error("PosFreeze with no value freezes (value defaults to 1)")
	}
	ctx.PosFreeze = false
	if err := applyCtrl(t, &ctx, "PosFreeze", map[string]string{"value": "0"}); err != nil {
		t.Fatal(err)
	}
	if ctx.PosFreeze {
		t.Error("value 0 does not freeze")
	}
}

// bytecode.go selfState.Run changes state within the fighter's own states,
// like ChangeState; the engine has no custom states, so they are the same.
func TestApplyController_SelfStateChangesStateLikeChangeState(t *testing.T) {
	var ctx evaluator.Context
	changed, err := ApplyController(cns.Controller{Type: "SelfState", Parameters: map[string]string{"value": "5110", "ctrl": "0"}}, &ctx, func(int) bool { return true })
	if err != nil || !changed || ctx.StateNo != 5110 || ctx.Ctrl {
		t.Fatalf("changed %v state %d ctrl %v err %v, want a change to 5110 without control", changed, ctx.StateNo, ctx.Ctrl, err)
	}
	if !IsSupportedController("selfstate") {
		t.Error("SelfState must be reported supported")
	}
}
