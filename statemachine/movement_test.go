package statemachine

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/match"
)

// controller builds a controller of the given type from alternating
// key, value parameter strings.
func controller(typ string, kv ...string) cns.Controller {
	params := map[string]string{}
	for i := 0; i < len(kv); i += 2 {
		params[kv[i]] = kv[i+1]
	}
	return cns.Controller{Type: typ, Parameters: params}
}

func fighterCtx(facing match.Facing, pos match.Position, vel match.Velocity) evaluator.Context {
	return evaluator.Context{FighterState: match.FighterState{Facing: facing, Position: pos, Velocity: vel, Health: 100}}
}

func apply(t *testing.T, c cns.Controller, ctx evaluator.Context) evaluator.Context {
	t.Helper()
	if _, err := ApplyController(c, &ctx, func(int) bool { return true }); err != nil {
		t.Fatalf("ApplyController(%s) returned unexpected error: %v", c.Type, err)
	}
	return ctx
}

func applyErr(t *testing.T, c cns.Controller, ctx evaluator.Context) error {
	t.Helper()
	_, err := ApplyController(c, &ctx, func(int) bool { return true })
	if err == nil {
		t.Fatalf("ApplyController(%s) returned nil, want a descriptive error", c.Type)
	}
	return err
}

func wantVel(t *testing.T, got match.Velocity, want match.Velocity) {
	t.Helper()
	if got != want {
		t.Errorf("Velocity = %+v, want %+v", got, want)
	}
}

func wantPos(t *testing.T, got match.Position, want match.Position) {
	t.Helper()
	if got != want {
		t.Errorf("Position = %+v, want %+v", got, want)
	}
}

// Ikemen's Y axis points down, the engine's up: a VelSet y of -8 is an
// upward 8, and facing-relative X becomes a signed world X.

func TestApplyController_VelSet_FacingRight_SetsForwardXAndFlipsY(t *testing.T) {
	ctx := apply(t, controller("VelSet", "x", "3", "y", "-8"), fighterCtx(match.FacingRight, match.Position{}, match.Velocity{X: 9, Y: 9}))
	wantVel(t, ctx.Velocity, match.Velocity{X: 3, Y: 8})
}

func TestApplyController_VelSet_FacingLeft_ForwardIsNegativeWorldX(t *testing.T) {
	ctx := apply(t, controller("VelSet", "x", "3"), fighterCtx(match.FacingLeft, match.Position{}, match.Velocity{X: 9, Y: 9}))
	wantVel(t, ctx.Velocity, match.Velocity{X: -3, Y: 9})
}

func TestApplyController_VelSet_OmittedAxis_IsLeftUntouched(t *testing.T) {
	ctx := apply(t, controller("VelSet", "y", "0"), fighterCtx(match.FacingRight, match.Position{}, match.Velocity{X: 4, Y: 4}))
	wantVel(t, ctx.Velocity, match.Velocity{X: 4, Y: 0})
}

func TestApplyController_VelSet_ExpressionParameter_IsEvaluated(t *testing.T) {
	ctx := fighterCtx(match.FacingRight, match.Position{}, match.Velocity{})
	ctx.Time = 4
	ctx = apply(t, controller("VelSet", "x", "Time * 0.5"), ctx)
	wantVel(t, ctx.Velocity, match.Velocity{X: 2})
}

func TestApplyController_VelSet_UnevaluableParameter_ReturnsErrorNamingIt(t *testing.T) {
	err := applyErr(t, controller("VelSet", "x", "1 +"), fighterCtx(match.FacingRight, match.Position{}, match.Velocity{}))
	if !strings.Contains(err.Error(), "VelSet") || !strings.Contains(err.Error(), "VelSet x ") {
		t.Errorf("error = %q, want it to name VelSet and parameter x", err)
	}
}

func TestApplyController_VelAdd_FacingLeft_AddsForwardXAndFlipsY(t *testing.T) {
	ctx := apply(t, controller("VelAdd", "x", "1", "y", "-2"), fighterCtx(match.FacingLeft, match.Position{}, match.Velocity{X: 2, Y: 1}))
	// forward 1 while facing left is world -1: 2 + (-1) = 1; up 2: 1 + 2 = 3.
	wantVel(t, ctx.Velocity, match.Velocity{X: 1, Y: 3})
}

func TestApplyController_VelAdd_FacingRight_AddsToCurrentVelocity(t *testing.T) {
	ctx := apply(t, controller("VelAdd", "x", "1.5"), fighterCtx(match.FacingRight, match.Position{}, match.Velocity{X: 2, Y: 1}))
	wantVel(t, ctx.Velocity, match.Velocity{X: 3.5, Y: 1})
}

func TestApplyController_VelMul_ScalesWorldVelocityRegardlessOfFacing(t *testing.T) {
	for _, facing := range []match.Facing{match.FacingRight, match.FacingLeft} {
		ctx := apply(t, controller("VelMul", "x", "0.5", "y", "2"), fighterCtx(facing, match.Position{}, match.Velocity{X: 4, Y: -6}))
		wantVel(t, ctx.Velocity, match.Velocity{X: 2, Y: -12})
	}
}

func TestApplyController_VelMul_ByZero_StopsTheAxis(t *testing.T) {
	ctx := apply(t, controller("VelMul", "x", "0"), fighterCtx(match.FacingRight, match.Position{}, match.Velocity{X: 4, Y: 5}))
	wantVel(t, ctx.Velocity, match.Velocity{X: 0, Y: 5})
}

func TestApplyController_PosSet_SetsWorldXAndFlipsY(t *testing.T) {
	for _, facing := range []match.Facing{match.FacingRight, match.FacingLeft} {
		ctx := apply(t, controller("PosSet", "x", "10", "y", "-20"), fighterCtx(facing, match.Position{X: 1, Y: 1}, match.Velocity{}))
		wantPos(t, ctx.Position, match.Position{X: 10, Y: 20})
	}
}

func TestApplyController_PosSet_OmittedAxis_IsLeftUntouched(t *testing.T) {
	ctx := apply(t, controller("PosSet", "y", "0"), fighterCtx(match.FacingRight, match.Position{X: 7, Y: 3}, match.Velocity{}))
	wantPos(t, ctx.Position, match.Position{X: 7, Y: 0})
}

func TestApplyController_PosAdd_FacingLeft_ForwardIsNegativeWorldX(t *testing.T) {
	ctx := apply(t, controller("PosAdd", "x", "5", "y", "-3"), fighterCtx(match.FacingLeft, match.Position{X: 50}, match.Velocity{}))
	wantPos(t, ctx.Position, match.Position{X: 45, Y: 3})
}

func TestApplyController_PosAdd_FacingRight_AddsToPosition(t *testing.T) {
	ctx := apply(t, controller("PosAdd", "x", "5"), fighterCtx(match.FacingRight, match.Position{X: 50, Y: 2}, match.Velocity{}))
	wantPos(t, ctx.Position, match.Position{X: 55, Y: 2})
}

func TestApplyController_PosAdd_UnevaluableParameter_ReturnsError(t *testing.T) {
	applyErr(t, controller("PosAdd", "y", "nosuchtrigger"), fighterCtx(match.FacingRight, match.Position{}, match.Velocity{}))
}

func TestApplyController_Gravity_AddsYAccelDownward(t *testing.T) {
	ctx := fighterCtx(match.FacingRight, match.Position{Y: 10}, match.Velocity{Y: 3})
	ctx.YAccel = 0.5
	ctx = apply(t, controller("Gravity"), ctx)
	wantVel(t, ctx.Velocity, match.Velocity{Y: 2.5})
}

func TestApplyController_Gravity_WithoutYAccel_ChangesNothing(t *testing.T) {
	ctx := apply(t, controller("Gravity"), fighterCtx(match.FacingRight, match.Position{}, match.Velocity{Y: 3}))
	wantVel(t, ctx.Velocity, match.Velocity{Y: 3})
}

func TestApplyController_Turn_FlipsFacingBothWays_KeepingWorldVelocity(t *testing.T) {
	ctx := apply(t, controller("Turn"), fighterCtx(match.FacingRight, match.Position{}, match.Velocity{X: 2}))
	if ctx.Facing != match.FacingLeft {
		t.Errorf("Facing = %v, want FacingLeft", ctx.Facing)
	}
	wantVel(t, ctx.Velocity, match.Velocity{X: 2})
	ctx = apply(t, controller("Turn"), ctx)
	if ctx.Facing != match.FacingRight {
		t.Errorf("Facing = %v, want FacingRight after a second Turn", ctx.Facing)
	}
}

func TestApplyController_ChangeAnim_SetsAnimAndRestartsAnimTime(t *testing.T) {
	ctx := evaluator.Context{Anim: 0, AnimElapsed: 7, AnimElem: 2}
	ctx = apply(t, controller("ChangeAnim", "value", "200"), ctx)
	if ctx.Anim != 200 || ctx.AnimElapsed != 0 {
		t.Errorf("Anim, AnimTime = %d, %d, want 200, 0", ctx.Anim, ctx.AnimElapsed)
	}
	if ctx.AnimElem != 0 {
		t.Errorf("AnimElem = %d, want 0 (a stale element request must not survive a plain ChangeAnim)", ctx.AnimElem)
	}
}

func TestApplyController_ChangeAnim_NegativeAnimBecomesZero_ButMinusTwoIsKept(t *testing.T) {
	ctx := apply(t, controller("ChangeAnim", "value", "-5"), evaluator.Context{Anim: 9})
	if ctx.Anim != 0 {
		t.Errorf("Anim = %d, want 0 for -5 (Ikemen char.go changeAnim)", ctx.Anim)
	}
	ctx = apply(t, controller("ChangeAnim", "value", "-2"), evaluator.Context{Anim: 9})
	if ctx.Anim != -2 {
		t.Errorf("Anim = %d, want -2 (Ikemen's invisible animation)", ctx.Anim)
	}
}

func TestApplyController_ChangeAnim_ElemAndElemTime_AreRecordedAsARequest(t *testing.T) {
	ctx := apply(t, controller("ChangeAnim", "value", "200", "elem", "3", "elemtime", "1"), evaluator.Context{})
	if ctx.AnimElem != 3 || ctx.AnimElemTime != 1 {
		t.Errorf("AnimElem, AnimElemTime = %d, %d, want 3, 1", ctx.AnimElem, ctx.AnimElemTime)
	}
}

func TestApplyController_ChangeAnim_ElemTimeWithoutElem_FallsBackToFirstElementAtTimeZero(t *testing.T) {
	// Ikemen char.go setAnimElem: an elem below 1 becomes 1 and drops elemtime.
	ctx := apply(t, controller("ChangeAnim", "value", "200", "elemtime", "2"), evaluator.Context{})
	if ctx.AnimElem != 1 || ctx.AnimElemTime != 0 {
		t.Errorf("AnimElem, AnimElemTime = %d, %d, want 1, 0", ctx.AnimElem, ctx.AnimElemTime)
	}
}

func TestApplyController_ChangeAnim_MissingValue_ReturnsDescriptiveError(t *testing.T) {
	err := applyErr(t, controller("ChangeAnim", "elem", "2"), evaluator.Context{})
	if !strings.Contains(err.Error(), `"value"`) {
		t.Errorf("error = %q, want it to name the missing value parameter", err)
	}
}

func TestApplyController_CtrlSet_SetsAndClearsCtrl(t *testing.T) {
	ctx := apply(t, controller("CtrlSet", "value", "0"), evaluator.Context{Ctrl: true})
	if ctx.Ctrl {
		t.Error("Ctrl = true after CtrlSet 0, want false")
	}
	ctx = apply(t, controller("CtrlSet", "value", "1"), ctx)
	if !ctx.Ctrl {
		t.Error("Ctrl = false after CtrlSet 1, want true")
	}
}

func TestApplyController_CtrlSet_ExpressionValue_IsEvaluated(t *testing.T) {
	ctx := apply(t, controller("CtrlSet", "value", "Time = 0"), evaluator.Context{Time: 3, Ctrl: true})
	if ctx.Ctrl {
		t.Error("Ctrl = true, want false (Time = 0 is false at Time 3)")
	}
}

func TestApplyController_CtrlSet_MissingValue_ReturnsDescriptiveError(t *testing.T) {
	err := applyErr(t, controller("CtrlSet"), evaluator.Context{})
	if !strings.Contains(err.Error(), `"value"`) {
		t.Errorf("error = %q, want it to name the missing value parameter", err)
	}
}

func TestApplyController_StateTypeSet_SetsEachGivenTypeCaseInsensitively(t *testing.T) {
	ctx := evaluator.Context{StateType: 'S', MoveType: 'I', Physics: 'N'}
	ctx = apply(t, controller("StateTypeSet", "statetype", "a", "movetype", "Hit", "physics", "a"), ctx)
	if ctx.StateType != 'A' || ctx.MoveType != 'H' || ctx.Physics != 'A' {
		t.Errorf("StateType, MoveType, Physics = %c, %c, %c, want A, H, A (first letter, upper-cased, as Ikemen stateTypeValue/moveTypeValue/physicsTypeValue)", ctx.StateType, ctx.MoveType, ctx.Physics)
	}
}

func TestApplyController_StateTypeSet_OmittedParameterAndU_LeaveTypeUnchanged(t *testing.T) {
	ctx := evaluator.Context{StateType: 'C', MoveType: 'A', Physics: 'S'}
	ctx = apply(t, controller("StateTypeSet", "movetype", "I"), ctx)
	if ctx.StateType != 'C' || ctx.MoveType != 'I' || ctx.Physics != 'S' {
		t.Errorf("StateType, MoveType, Physics = %c, %c, %c, want C, I, S", ctx.StateType, ctx.MoveType, ctx.Physics)
	}
	ctx = apply(t, controller("StateTypeSet", "statetype", "U", "movetype", "U", "physics", "U"), ctx)
	if ctx.StateType != 'C' || ctx.MoveType != 'I' || ctx.Physics != 'S' {
		t.Errorf("StateType, MoveType, Physics = %c, %c, %c after U, want C, I, S unchanged", ctx.StateType, ctx.MoveType, ctx.Physics)
	}
}

func TestApplyController_StateTypeSet_InvalidType_ReturnsDescriptiveError(t *testing.T) {
	for _, c := range []cns.Controller{
		controller("StateTypeSet", "statetype", "X"),
		controller("StateTypeSet", "movetype", "Z"),
		controller("StateTypeSet", "physics", "Q"),
		controller("StateTypeSet", "statetype", ""),
	} {
		applyErr(t, c, evaluator.Context{})
	}
}

func lifeCtx(health int) evaluator.Context {
	return evaluator.Context{FighterState: match.FighterState{Health: health}}
}

func TestApplyController_LifeAdd_AppliesDamageAndHealing(t *testing.T) {
	ctx := apply(t, controller("LifeAdd", "value", "-30"), lifeCtx(100))
	if ctx.Health != 70 {
		t.Errorf("Health = %d, want 70", ctx.Health)
	}
	ctx = apply(t, controller("LifeAdd", "value", "20"), ctx)
	if ctx.Health != 90 {
		t.Errorf("Health = %d, want 90", ctx.Health)
	}
}

func TestApplyController_LifeAdd_KillDefaultsToTrue_SoLethalDamageKOs(t *testing.T) {
	ctx := apply(t, controller("LifeAdd", "value", "-200"), lifeCtx(100))
	if ctx.Health != 0 {
		t.Errorf("Health = %d, want 0 (never negative)", ctx.Health)
	}
}

func TestApplyController_LifeAdd_KillZero_LeavesOneHealth(t *testing.T) {
	ctx := apply(t, controller("LifeAdd", "value", "-200", "kill", "0"), lifeCtx(100))
	if ctx.Health != 1 {
		t.Errorf("Health = %d, want 1 (kill = 0 cannot KO, Ikemen char.go lifeAdd)", ctx.Health)
	}
}

func TestApplyController_LifeAdd_ClampsHealingAtMaxHealth(t *testing.T) {
	ctx := apply(t, controller("LifeAdd", "value", "100"), lifeCtx(DefaultMaxHealth-50))
	if ctx.Health != DefaultMaxHealth {
		t.Errorf("Health = %d, want %d", ctx.Health, DefaultMaxHealth)
	}
}

func TestApplyController_LifeAdd_OnKOdFighter_NeverRevives(t *testing.T) {
	for _, c := range []cns.Controller{
		controller("LifeAdd", "value", "50"),
		controller("LifeAdd", "value", "-50", "kill", "0"),
		controller("LifeAdd", "value", "50", "absolute", "1"),
	} {
		if ctx := apply(t, c, lifeCtx(0)); ctx.Health != 0 {
			t.Errorf("%v: Health = %d, want 0 (a KO'd fighter is never revived by accident)", c.Parameters, ctx.Health)
		}
	}
}

func TestApplyController_LifeAdd_AbsoluteParameter_IsAccepted(t *testing.T) {
	ctx := apply(t, controller("LifeAdd", "value", "-10", "absolute", "1"), lifeCtx(100))
	if ctx.Health != 90 {
		t.Errorf("Health = %d, want 90 (no defense stat: absolute and relative agree)", ctx.Health)
	}
}

func TestApplyController_LifeAdd_ZeroValue_ChangesNothing(t *testing.T) {
	ctx := apply(t, controller("LifeAdd", "value", "0", "kill", "0"), lifeCtx(50))
	if ctx.Health != 50 {
		t.Errorf("Health = %d, want 50", ctx.Health)
	}
}

func TestApplyController_LifeAdd_ExactlyLethal_KillsByDefaultAndLeavesOneWithKillZero(t *testing.T) {
	if got := apply(t, controller("LifeAdd", "value", "-100"), lifeCtx(100)).Health; got != 0 {
		t.Errorf("default kill: Health = %d, want 0", got)
	}
	if got := apply(t, controller("LifeAdd", "value", "-100", "kill", "0"), lifeCtx(100)).Health; got != 1 {
		t.Errorf("kill = 0: Health = %d, want 1", got)
	}
	if got := apply(t, controller("LifeAdd", "value", "-99", "kill", "0"), lifeCtx(100)).Health; got != 1 {
		t.Errorf("kill = 0, leaves 1 naturally: Health = %d, want 1", got)
	}
}

func TestApplyController_QuotedAndLowerCaseTypeLetters_AreAccepted(t *testing.T) {
	ctx := apply(t, controller("StateTypeSet", "statetype", `"a"`, "movetype", "h"), evaluator.Context{StateType: 'S', MoveType: 'I'})
	if ctx.StateType != 'A' || ctx.MoveType != 'H' {
		t.Errorf("StateType, MoveType = %c, %c, want A, H", ctx.StateType, ctx.MoveType)
	}
}

// Every controller that evaluates a parameter names itself and the
// parameter when that expression fails.
func TestApplyController_UnevaluableParameter_ErrorNamesControllerAndParameter(t *testing.T) {
	tests := []struct {
		typ, key string
		extra    []string
	}{
		{"VelSet", "x", nil}, {"VelSet", "y", nil},
		{"VelAdd", "x", nil}, {"VelAdd", "y", nil},
		{"VelMul", "x", nil}, {"VelMul", "y", nil},
		{"PosSet", "x", nil}, {"PosSet", "y", nil},
		{"PosAdd", "x", nil}, {"PosAdd", "y", nil},
		{"ChangeAnim", "value", nil}, {"ChangeAnim", "elem", []string{"value", "1"}}, {"ChangeAnim", "elemtime", []string{"value", "1"}},
		{"CtrlSet", "value", nil},
		{"LifeAdd", "value", nil}, {"LifeAdd", "kill", []string{"value", "1"}},
		{"LifeSet", "value", nil},
	}
	for _, tc := range tests {
		c := controller(tc.typ, append([]string{tc.key, "1 +"}, tc.extra...)...)
		err := applyErr(t, c, lifeCtx(10))
		if want := tc.typ + " " + tc.key + " "; !strings.Contains(err.Error(), want) {
			t.Errorf("%s %s: error = %q, want it to contain %q", tc.typ, tc.key, err, want)
		}
	}
}

func TestApplyController_NegativeZeroNeverProduced(t *testing.T) {
	left := fighterCtx(match.FacingLeft, match.Position{}, match.Velocity{})
	right := fighterCtx(match.FacingRight, match.Position{}, match.Velocity{X: 4, Y: 4})
	tests := []struct {
		name string
		got  float64
	}{
		{"VelSet x=0 facing left", apply(t, controller("VelSet", "x", "0"), left).Velocity.X},
		{"VelSet y=0", apply(t, controller("VelSet", "y", "0"), left).Velocity.Y},
		{"VelAdd x=0 facing left", apply(t, controller("VelAdd", "x", "0"), left).Velocity.X},
		{"PosAdd x=0 facing left", apply(t, controller("PosAdd", "x", "0"), left).Position.X},
		{"PosAdd y=0", apply(t, controller("PosAdd", "y", "0"), left).Position.Y},
		{"PosSet x=-0", apply(t, controller("PosSet", "x", "-0"), left).Position.X},
		{"VelMul x by -0", apply(t, controller("VelMul", "x", "-0"), right).Velocity.X},
		{"VelMul y by -0", apply(t, controller("VelMul", "y", "-0"), right).Velocity.Y},
	}
	for _, tc := range tests {
		if tc.got != 0 || math.Signbit(tc.got) {
			t.Errorf("%s: got %v (signbit %v), want +0", tc.name, tc.got, math.Signbit(tc.got))
		}
	}
}

func TestApplyController_LifeAdd_MissingValue_ReturnsDescriptiveError(t *testing.T) {
	err := applyErr(t, controller("LifeAdd", "kill", "0"), lifeCtx(10))
	if !strings.Contains(err.Error(), `"value"`) {
		t.Errorf("error = %q, want it to name the missing value parameter", err)
	}
}

func TestApplyController_LifeSet_SetsAndClamps(t *testing.T) {
	tests := []struct {
		value string
		want  int
	}{
		{"500", 500},
		{"5000", DefaultMaxHealth},
		{"-3", 0},
	}
	for _, tc := range tests {
		ctx := apply(t, controller("LifeSet", "value", tc.value), lifeCtx(100))
		if ctx.Health != tc.want {
			t.Errorf("LifeSet %s: Health = %d, want %d", tc.value, ctx.Health, tc.want)
		}
	}
}

func TestApplyController_LifeSet_OnKOdFighter_NeverRevives(t *testing.T) {
	ctx := apply(t, controller("LifeSet", "value", "300"), lifeCtx(0))
	if ctx.Health != 0 {
		t.Errorf("Health = %d, want 0", ctx.Health)
	}
}

func TestApplyController_LifeSet_MissingValue_ReturnsDescriptiveError(t *testing.T) {
	applyErr(t, controller("LifeSet"), lifeCtx(10))
}

func TestApplyController_Null_IsSupportedAndChangesNothing(t *testing.T) {
	before := fighterCtx(match.FacingLeft, match.Position{X: 1, Y: 2}, match.Velocity{X: 3, Y: 4})
	after := apply(t, controller("Null"), before)
	if !reflect.DeepEqual(before, after) {
		t.Errorf("Null changed the context: %+v -> %+v", before, after)
	}
	if len(after.Unsupported) != 0 {
		t.Errorf("Unsupported = %v, want none: Null is supported", after.Unsupported)
	}
}

func TestIsSupportedController_MovementAndStateControllers(t *testing.T) {
	for _, typ := range []string{"VelSet", "veladd", "VELMUL", "PosSet", "PosAdd", "ChangeAnim", "CtrlSet", "StateTypeSet", "Turn", "LifeAdd", "LifeSet", "Gravity", "Null"} {
		if !IsSupportedController(typ) {
			t.Errorf("IsSupportedController(%q) = false, want true", typ)
		}
	}
	if IsSupportedController("MoveTypeSet") {
		t.Error("MoveTypeSet is not an Ikemen GO controller (StateTypeSet sets movetype): it must not be registered")
	}
}

func TestStep_VelSetAndPosAdd_ApplyOnTheTickTheirTriggersHold(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {Number: 0, Controllers: []cns.Controller{
			{Type: "VelSet", Triggers: []string{"Time = 0"}, Parameters: map[string]string{"x": "4"}},
			{Type: "PosAdd", Triggers: []string{"Time = 1"}, Parameters: map[string]string{"x": "10"}},
		}},
	}
	ctx := fighterCtx(match.FacingRight, match.Position{}, match.Velocity{})

	first, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	wantVel(t, first.Context.Velocity, match.Velocity{X: 4})
	wantPos(t, first.Context.Position, match.Position{})

	ctx.Time = 1
	second, err := Step(ctx, states)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	wantVel(t, second.Context.Velocity, match.Velocity{})
	wantPos(t, second.Context.Position, match.Position{X: 10})
}

func unsupportedStates() map[int]cns.StateDef {
	return map[int]cns.StateDef{
		0: {Number: 0, Controllers: []cns.Controller{
			{Type: "BindToRoot", Triggers: []string{"1"}},
			{Type: "bindtoroot", Triggers: []string{"1"}},
			{Type: "ReversalDef", Triggers: []string{"1"}},
			{Type: "Projectile", Triggers: []string{"0"}},
			{Type: "HitDef", Triggers: []string{"1"}},
		}},
	}
}

func TestStep_UnsupportedControllerTypes_AreReportedByNameOncePerCall(t *testing.T) {
	result, err := Step(evaluator.Context{}, unsupportedStates())
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if want := []string{"bindtoroot", "reversaldef"}; !reflect.DeepEqual(result.Unsupported, want) {
		t.Errorf("Unsupported = %v, want %v (lower-cased, deduplicated; false-trigger Projectile and Tick-handled HitDef absent)", result.Unsupported, want)
	}
	if want := []int{0, 1, 2, 4}; !intSlicesEqual(result.Applied, want) {
		t.Errorf("Applied = %v, want %v (unsupported controllers still count as triggered)", result.Applied, want)
	}
	if len(result.Context.Unsupported) != 0 {
		t.Errorf("Context.Unsupported = %v, want empty: it is a per-call log, not fighter state", result.Context.Unsupported)
	}
}

func TestStep_Unsupported_DoesNotLeakAcrossCalls(t *testing.T) {
	first, err := Step(evaluator.Context{}, unsupportedStates())
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	ctx := first.Context
	ctx.Unsupported = []string{"stale"}
	quiet := map[int]cns.StateDef{0: {Number: 0}}
	second, err := Step(ctx, quiet)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if second.Unsupported != nil {
		t.Errorf("Unsupported = %v, want nil on a call with no unsupported controller", second.Unsupported)
	}
}

func TestStep_NoUnsupportedController_ReportsNone(t *testing.T) {
	states := map[int]cns.StateDef{0: {Number: 0, Controllers: []cns.Controller{controller("Null")}}}
	result, err := Step(evaluator.Context{}, states)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if result.Unsupported != nil {
		t.Errorf("Unsupported = %v, want nil", result.Unsupported)
	}
}

func TestApplyController_VelAddAfterTurn_UsesTheNewFacing(t *testing.T) {
	ctx := fighterCtx(match.FacingRight, match.Position{}, match.Velocity{})
	ctx = apply(t, controller("VelSet", "x", "2.5"), ctx)
	ctx = apply(t, controller("Turn"), ctx)
	ctx = apply(t, controller("VelAdd", "x", "2.5"), ctx)
	// world X was +2.5; after Turn, forward 2.5 is world -2.5: sum 0.
	if math.Abs(ctx.Velocity.X) > 1e-12 {
		t.Errorf("Velocity.X = %v, want 0", ctx.Velocity.X)
	}
}

// Ikemen GO bytecode.go:11309-11327 (playerPush.Run): value sets the
// playerpush flag, priority and affectteam the push properties.
func TestApplyController_PlayerPush_SetsTheFlagPriorityAndAffectTeam(t *testing.T) {
	ctx := evaluator.Context{PlayerPush: true, PushAffectTeam: 1}
	ctx = apply(t, controller("PlayerPush", "value", "0"), ctx)
	if ctx.PlayerPush {
		t.Error("PlayerPush = true after PlayerPush value = 0, want false")
	}
	ctx = apply(t, controller("PlayerPush", "value", "1", "priority", "2", "affectteam", "F"), ctx)
	if !ctx.PlayerPush || ctx.PushPriority != 2 || ctx.PushAffectTeam != -1 {
		t.Errorf("PlayerPush, PushPriority, PushAffectTeam = %v, %d, %d, want true, 2, -1", ctx.PlayerPush, ctx.PushPriority, ctx.PushAffectTeam)
	}
	// B (both) is 0, E (enemy) 1 (compiler_functions.go:3570-3577).
	ctx = apply(t, controller("PlayerPush", "affectteam", "b"), ctx)
	if !ctx.PlayerPush || ctx.PushAffectTeam != 0 {
		t.Errorf("PlayerPush, PushAffectTeam = %v, %d after affectteam = b alone, want true (unchanged), 0", ctx.PlayerPush, ctx.PushAffectTeam)
	}
}

// compiler_functions.go:3546-3590: at least one parameter, and affectteam
// starting with E, F or B.
func TestApplyController_PlayerPush_NoParameterOrBadAffectTeam_ReturnsDescriptiveError(t *testing.T) {
	if err := applyErr(t, controller("PlayerPush"), evaluator.Context{}); !strings.Contains(err.Error(), "at least one") {
		t.Errorf("error = %v, want it to ask for at least one parameter", err)
	}
	if err := applyErr(t, controller("PlayerPush", "affectteam", "X"), evaluator.Context{}); !strings.Contains(err.Error(), "affectteam") {
		t.Errorf("error = %v, want it to name affectteam", err)
	}
}
