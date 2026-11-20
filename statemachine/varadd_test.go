package statemachine

import (
	"strings"
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
)

func applyCtrl(t *testing.T, ctx *evaluator.Context, typ string, params map[string]string) error {
	t.Helper()
	_, err := ApplyController(cns.Controller{Type: typ, Parameters: params}, ctx, func(int) bool { return true })
	return err
}

// compiler_functions.go:2889-2898: VarAdd takes VarSet's parameter forms
// (varSetSub with add), adding the value to the variable.
func TestApplyController_VarAddAddsInEveryForm(t *testing.T) {
	var ctx evaluator.Context
	ctx.Vars[3], ctx.FVars[1], ctx.SysVars[2], ctx.SysFVars[0] = 10, 1.5, 7, 0.25
	for _, p := range []map[string]string{
		{"var(3)": "5"}, {"fvar(1)": "0.5"}, {"sysvar(2)": "-2"}, {"sysfv": "0", "value": "0.5"},
	} {
		if err := applyCtrl(t, &ctx, "VarAdd", p); err != nil {
			t.Fatalf("%v: %v", p, err)
		}
	}
	if ctx.Vars[3] != 15 || ctx.FVars[1] != 2 || ctx.SysVars[2] != 5 || ctx.SysFVars[0] != 0.75 {
		t.Errorf("vars = %d %v %d %v, want 15 2 5 0.75", ctx.Vars[3], ctx.FVars[1], ctx.SysVars[2], ctx.SysFVars[0])
	}
	if err := applyCtrl(t, &ctx, "VarAdd", map[string]string{"var(99)": "1"}); err == nil {
		t.Error("var(99): want an index error")
	}
}

// bytecode.go:12025-12050 and char.go:8424-8476: VarRangeSet sets var(first)
// to var(last) to value, or fvar(first..last) to fvalue; first defaults to 0
// and last to 59; a range with first < 0 or first > last sets nothing.
func TestApplyController_VarRangeSet(t *testing.T) {
	var ctx evaluator.Context
	if err := applyCtrl(t, &ctx, "VarRangeSet", map[string]string{"first": "2", "last": "4", "value": "9"}); err != nil {
		t.Fatal(err)
	}
	if ctx.Vars[1] != 0 || ctx.Vars[2] != 9 || ctx.Vars[4] != 9 || ctx.Vars[5] != 0 {
		t.Errorf("vars 1..5 = %v, want 0 9 9 9 0", ctx.Vars[1:6])
	}
	if err := applyCtrl(t, &ctx, "VarRangeSet", map[string]string{"value": "1"}); err != nil {
		t.Fatal(err)
	}
	if ctx.Vars[0] != 1 || ctx.Vars[59] != 1 {
		t.Errorf("default range: var(0) %d var(59) %d, want both 1", ctx.Vars[0], ctx.Vars[59])
	}
	if err := applyCtrl(t, &ctx, "VarRangeSet", map[string]string{"first": "1", "last": "2", "fvalue": "0.5"}); err != nil {
		t.Fatal(err)
	}
	if ctx.FVars[1] != 0.5 || ctx.FVars[2] != 0.5 || ctx.FVars[3] != 0 {
		t.Errorf("fvars 1..3 = %v, want 0.5 0.5 0", ctx.FVars[1:4])
	}
	before := ctx.Vars
	if err := applyCtrl(t, &ctx, "VarRangeSet", map[string]string{"first": "5", "last": "2", "value": "3"}); err != nil {
		t.Fatal(err)
	}
	if err := applyCtrl(t, &ctx, "VarRangeSet", map[string]string{"first": "-1", "last": "2", "value": "3"}); err != nil {
		t.Fatal(err)
	}
	if ctx.Vars != before {
		t.Error("an invalid range must set nothing")
	}
	if err := applyCtrl(t, &ctx, "VarRangeSet", map[string]string{"first": "0"}); err == nil {
		t.Error("no value nor fvalue: want an error")
	}
}

// char.go lifeSet / lifeAdd clamp to lifeMax, the character's [Data] life
// (char.go:4615-4630 reads data.life), not a constant.
func TestApplyController_LifeClampsToTheCharactersLife(t *testing.T) {
	c, err := evaluator.ParseConstants(strings.NewReader("[Data]\nlife = 1200\n"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := evaluator.Context{Consts: c}
	ctx.Health = 1000
	if err := applyCtrl(t, &ctx, "LifeAdd", map[string]string{"value": "500"}); err != nil {
		t.Fatal(err)
	}
	if ctx.Health != 1200 {
		t.Errorf("LifeAdd 500 from 1000 = %d, want the character's 1200", ctx.Health)
	}
	if err := applyCtrl(t, &ctx, "LifeSet", map[string]string{"value": "5000"}); err != nil {
		t.Fatal(err)
	}
	if ctx.Health != 1200 {
		t.Errorf("LifeSet 5000 = %d, want 1200", ctx.Health)
	}
}
