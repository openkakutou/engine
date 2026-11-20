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

// compiler_functions.go hitBySub and bytecode.go:5517-5566: value fills slot
// 0, value2 slot 1 (ignored with value), attr needs a slot; time defaults to
// 1.
func TestApplyController_HitByAndNotHitBy(t *testing.T) {
	var ctx evaluator.Context
	if err := applyCtrl(t, &ctx, "NotHitBy", map[string]string{"value": "SCA", "time": "5"}); err != nil {
		t.Fatal(err)
	}
	if s := ctx.HitBy[0]; !s.Not || s.Time != 5 || s.Attr == 0 {
		t.Errorf("slot 0 = %+v, want a 5-tick NotHitBy", s)
	}
	if err := applyCtrl(t, &ctx, "HitBy", map[string]string{"value2": "S, NA"}); err != nil {
		t.Fatal(err)
	}
	if s := ctx.HitBy[1]; s.Not || s.Time != 1 {
		t.Errorf("slot 1 = %+v, want a 1-tick HitBy", s)
	}
	if err := applyCtrl(t, &ctx, "HitBy", map[string]string{"attr": "A, AA", "slot": "3", "stack": "1"}); err != nil {
		t.Fatal(err)
	}
	if s := ctx.HitBy[3]; !s.Stack || s.Time != 1 {
		t.Errorf("slot 3 = %+v, want a stacked HitBy", s)
	}
	before := ctx.HitBy
	if err := applyCtrl(t, &ctx, "HitBy", map[string]string{"attr": "SCA"}); err != nil {
		t.Fatal(err)
	}
	if ctx.HitBy != before {
		t.Error("attr without slot sets nothing (bytecode.go:5528-5529)")
	}
	if err := applyCtrl(t, &ctx, "NotHitBy", map[string]string{"time": "3"}); err == nil {
		t.Error("no attribute at all: want an error")
	}
}

// bytecode.go:11851-11888 defenceMulSet: a MUGEN character's defaults are
// onHit 1 and mulType 0 (defence = 1 / value), an Ikemen GO one's onHit 0 and
// mulType 1 (defence = value).
func TestApplyController_DefenceMulSet(t *testing.T) {
	var mugen evaluator.Context
	if err := applyCtrl(t, &mugen, "DefenceMulSet", map[string]string{"value": "2"}); err != nil {
		t.Fatal(err)
	}
	if mugen.CustomDefence != 0.5 || !mugen.DefenceMulOnHit {
		t.Errorf("MUGEN character: defence %v onHit %v, want 0.5 true", mugen.CustomDefence, mugen.DefenceMulOnHit)
	}
	ikemen := evaluator.Context{IkemenChar: true}
	if err := applyCtrl(t, &ikemen, "DefenceMulSet", map[string]string{"value": "2"}); err != nil {
		t.Fatal(err)
	}
	if ikemen.CustomDefence != 2 || ikemen.DefenceMulOnHit {
		t.Errorf("Ikemen character: defence %v onHit %v, want 2 false", ikemen.CustomDefence, ikemen.DefenceMulOnHit)
	}
	if err := applyCtrl(t, &ikemen, "DefenceMulSet", map[string]string{"value": "4", "multype": "0", "onhit": "1"}); err != nil {
		t.Fatal(err)
	}
	if ikemen.CustomDefence != 0.25 || !ikemen.DefenceMulOnHit {
		t.Errorf("explicit mulType 0 onHit 1: %v %v, want 0.25 true", ikemen.CustomDefence, ikemen.DefenceMulOnHit)
	}
}

// bytecode.go:5604-5632 assertSpecial.Run: flag, flag2 and flag3 are set,
// or cleared with enabled 0.
func TestApplyController_AssertSpecial(t *testing.T) {
	var ctx evaluator.Context
	if err := applyCtrl(t, &ctx, "AssertSpecial", map[string]string{"flag": "nowalk", "flag2": "NoJump", "flag3": "intro"}); err != nil {
		t.Fatal(err)
	}
	if !ctx.Asserted.Has("nowalk") || !ctx.Asserted.Has("nojump") || !ctx.Asserted.Has("intro") {
		t.Errorf("asserted = %v, want nowalk, nojump, intro", ctx.Asserted)
	}
	if err := applyCtrl(t, &ctx, "AssertSpecial", map[string]string{"flag": "nowalk", "enabled": "0"}); err != nil {
		t.Fatal(err)
	}
	if ctx.Asserted.Has("nowalk") {
		t.Error("enabled 0 clears nowalk")
	}
	if err := applyCtrl(t, &ctx, "AssertSpecial", map[string]string{"flag": "fly"}); err == nil {
		t.Error("an unknown flag must be an error")
	}
}
