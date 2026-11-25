package statemachine

import (
	"strings"
	"testing"

	"github.com/openkakutou/engine/v2/match"
)

// A controller parameter that does not evaluate fails the controller with an
// error naming the controller and the parameter, instead of a silent 0.
func TestApplyController_UnevaluableParameter_FailsNamingIt(t *testing.T) {
	const bad = "1 +"
	for _, c := range []struct {
		typ, param string
		others     []string // the other parameters the controller needs
		helper     bool     // the controller only acts in a helper
	}{
		{"HitBy", "time", []string{"value", "SCA"}, false},
		{"HitBy", "slot", []string{"attr", "SCA"}, false},
		{"HitBy", "stack", []string{"value", "SCA"}, false},
		{"NotHitBy", "time", []string{"value", "SCA"}, false},
		{"HitVelSet", "x", nil, false},
		{"HitVelSet", "y", nil, false},
		{"HitFallSet", "value", nil, false},
		{"HitFallSet", "xvel", nil, false},
		{"HitFallSet", "yvel", nil, false},
		{"PosFreeze", "value", nil, false},
		{"DefenceMulSet", "value", nil, false},
		{"DefenceMulSet", "onhit", []string{"value", "1"}, false},
		{"DefenceMulSet", "multype", []string{"value", "1"}, false},
		{"AssertSpecial", "enabled", []string{"flag", "nowalk"}, false},
		{"Pause", "time", nil, false},
		{"Pause", "movetime", []string{"time", "10"}, false},
		{"Pause", "poweradd", []string{"time", "10"}, false},
		{"SuperPause", "time", nil, false},
		{"SuperPause", "movetime", nil, false},
		{"SuperPause", "poweradd", nil, false},
		{"SuperPause", "p2defmul", nil, false},
		{"SuperPause", "darken", nil, false},
		{"SuperPause", "unhittable", nil, false},
		{"SuperPause", "anim", nil, false},
		{"SuperPause", "pos", nil, false},
		{"Projectile", "projid", nil, false},
		{"Projectile", "projanim", nil, false},
		{"Projectile", "projhitanim", nil, false},
		{"Projectile", "projremanim", nil, false},
		{"Projectile", "projcancelanim", nil, false},
		{"Projectile", "projremovetime", nil, false},
		{"Projectile", "projmisstime", nil, false},
		{"Projectile", "projhits", nil, false},
		{"Projectile", "projpriority", nil, false},
		{"Projectile", "projremove", nil, false},
		{"Projectile", "velocity", nil, false},
		{"Projectile", "velmul", nil, false},
		{"Projectile", "remvelocity", nil, false},
		{"Projectile", "accel", nil, false},
		{"Projectile", "offset", nil, false},
		{"ChangeAnim2", "value", nil, false},
		{"BindToTarget", "time", nil, false},
		{"BindToTarget", "pos", nil, false},
		{"TargetBind", "time", nil, false},
		{"TargetBind", "pos", nil, false},
		{"TargetFacing", "value", nil, false},
		{"TargetState", "value", nil, false},
		{"TargetLifeAdd", "value", nil, false},
		{"TargetLifeAdd", "kill", []string{"value", "10"}, false},
		{"TargetLifeAdd", "absolute", []string{"value", "10"}, false},
		{"TargetPowerAdd", "value", nil, false},
		{"TargetVelSet", "x", nil, false},
		{"TargetVelSet", "y", nil, false},
		{"TargetVelAdd", "x", nil, false},
		{"TargetVelAdd", "y", nil, false},
		{"HitOverride", "slot", []string{"attr", "SCA"}, false},
		{"HitOverride", "stateno", []string{"attr", "SCA"}, false},
		{"HitOverride", "time", []string{"attr", "SCA"}, false},
		{"HitOverride", "keepstate", []string{"attr", "SCA"}, false},
		{"BindToParent", "time", nil, true},
		{"BindToParent", "facing", nil, true},
		{"BindToParent", "pos", nil, true},
		{"BindToRoot", "time", nil, true},
		{"BindToRoot", "facing", nil, true},
		{"BindToRoot", "pos", nil, true},
		{"Helper", "keyctrl", nil, false},
		{"Helper", "pos", nil, false},
		{"Explod", "pos", []string{"anim", "1"}, false},
		{"Explod", "vel", []string{"anim", "1"}, false},
		{"Explod", "accel", []string{"anim", "1"}, false},
		{"Explod", "scale", []string{"anim", "1"}, false},
		{"Trans", "alpha", []string{"trans", "addalpha"}, false},
		{"Width", "edge", nil, false},
		{"ScreenBound", "movecamera", nil, false},
		{"AngleDraw", "scale", nil, false},
		{"VarRandom", "range", []string{"v", "1"}, false},
		{"AttackDist", "x", nil, false},
	} {
		kv := append([]string{c.param, bad}, c.others...)
		ctx := fighterCtx(match.FacingRight, match.Position{}, match.Velocity{})
		ctx.IsHelper = c.helper
		_, err := ApplyController(controller(c.typ, kv...), &ctx, func(int) bool { return true })
		if err == nil {
			t.Errorf("%s %s = %q: no error, want one naming the controller and the parameter", c.typ, c.param, bad)
			continue
		}
		if !strings.Contains(err.Error(), c.typ) || !strings.Contains(err.Error(), c.param) {
			t.Errorf("%s %s = %q: error %q does not name the controller and the parameter", c.typ, c.param, bad, err)
		}
	}
}
