package statemachine

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/match"
)

func applyVarSetParams(t *testing.T, ctx evaluator.Context, params map[string]string) (evaluator.Context, error) {
	t.Helper()
	_, err := ApplyController(cns.Controller{Type: "VarSet", Parameters: params}, &ctx, func(int) bool { return true })
	return ctx, err
}

func TestApplyController_VarSetVarIndexForm_SetsVariable(t *testing.T) {
	ctx, err := applyVarSetParams(t, evaluator.Context{}, map[string]string{"var(3)": "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Vars[3] != 1 {
		t.Errorf("Vars[3] = %d, want 1", ctx.Vars[3])
	}
}

// Ikemen GO varSetSub (src/compiler_functions.go:2814-2876): the index is a
// full expression.
func TestApplyController_VarSetIndexExpression_IsEvaluated(t *testing.T) {
	ctx, err := applyVarSetParams(t, evaluator.Context{}, map[string]string{"var(1+2)": "5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Vars[3] != 5 {
		t.Errorf("Vars[3] = %d, want 5", ctx.Vars[3])
	}
}

func TestApplyController_VarSetValueExpression_IsEvaluated(t *testing.T) {
	in := evaluator.Context{}
	in.Vars[0] = 40
	ctx, err := applyVarSetParams(t, in, map[string]string{"var(3)": "var(0)+2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Vars[3] != 42 {
		t.Errorf("Vars[3] = %d, want 42", ctx.Vars[3])
	}
}

func TestApplyController_VarSetSysvarForm_SetsSysVar(t *testing.T) {
	ctx, err := applyVarSetParams(t, evaluator.Context{}, map[string]string{"sysvar(2)": "7"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.SysVars[2] != 7 || ctx.Vars[2] != 0 {
		t.Errorf("SysVars[2] = %d, Vars[2] = %d, want 7 and 0", ctx.SysVars[2], ctx.Vars[2])
	}
}

// The parser lower-cases keys but keeps inner spaces; Ikemen GO allows
// spaces before "(" (compiler_functions.go:2814-2876).
func TestApplyController_VarSetParsedMixedCaseAndSpacedKey_SetsVariable(t *testing.T) {
	defs, err := cns.Parse(strings.NewReader("[Statedef 0]\n[State 0, a]\ntype = VarSet\ntrigger1 = 1\nVar (3) = 9\n"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := evaluator.Context{}
	if _, err := ApplyController(defs[0].Controllers[0], &ctx, func(int) bool { return true }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Vars[3] != 9 {
		t.Errorf("Vars[3] = %d, want 9", ctx.Vars[3])
	}
}

func TestApplyController_VarSetOldVValueForm_StillSetsVar(t *testing.T) {
	ctx, err := applyVarSetParams(t, evaluator.Context{}, map[string]string{"v": "4", "value": "6"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Vars[4] != 6 {
		t.Errorf("Vars[4] = %d, want 6", ctx.Vars[4])
	}
}

func TestApplyController_VarSetInvalidForms_ReturnClearErrors(t *testing.T) {
	cases := []struct {
		name   string
		params map[string]string
		want   string
	}{
		{"negative index", map[string]string{"var(-1)": "1"}, "out of range"},
		{"var index too high", map[string]string{"var(60)": "1"}, "out of range"},
		{"sysvar index too high", map[string]string{"sysvar(5)": "1"}, "out of range"},
		{"empty index", map[string]string{"var()": "1"}, "Missing variable index"},
		{"two var forms", map[string]string{"var(1)": "1", "sysvar(2)": "1"}, "only set one variable at a time"},
		{"var form with v", map[string]string{"var(1)": "1", "v": "2", "value": "3"}, "only set one variable at a time"},
		{"no variable", map[string]string{"foo": "1"}, "VarSet needs a variable"},
		{"v without value", map[string]string{"v": "3"}, `"value" parameter`},
		{"value without v", map[string]string{"value": "3"}, "VarSet needs a variable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := applyVarSetParams(t, evaluator.Context{}, tc.params)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want one containing %q", err, tc.want)
			}
		})
	}
}

func TestApplyController_VarSetFloatForms_ReturnFloatVarUnsupported(t *testing.T) {
	for _, key := range []string{"fvar(1)", "sysfvar(0)"} {
		_, err := applyVarSetParams(t, evaluator.Context{}, map[string]string{key: "1"})
		if !errors.Is(err, ErrFloatVarUnsupported) {
			t.Errorf("%s: error = %v, want ErrFloatVarUnsupported", key, err)
		}
	}
}

// Every VarSet in the real common1.cns and kfm.cns applies. A controller
// whose value uses a trigger the evaluator does not know is skipped (the
// error then comes from the evaluator, not the VarSet forms).
func TestApplyController_RealFixtureVarSets_ApplyWithoutError(t *testing.T) {
	for _, path := range []string{"../testdata/reference/common1.cns", "../testdata/reference/chars/kfm/kfm.cns"} {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defs, err := cns.Parse(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		applied := 0
		for _, def := range defs {
			for _, ctrl := range def.Controllers {
				if !strings.EqualFold(ctrl.Type, "VarSet") {
					continue
				}
				ctx := evaluator.Context{FighterState: match.FighterState{StateNo: def.Number}}
				_, err := ApplyController(ctrl, &ctx, func(int) bool { return true })
				if err != nil && strings.Contains(err.Error(), "VarSet needs a variable") {
					t.Errorf("%s state %d: %v (params %v)", path, def.Number, err, ctrl.Parameters)
				} else if err != nil && !strings.Contains(err.Error(), "unknown") {
					t.Errorf("%s state %d: %v", path, def.Number, err)
				}
				applied++
			}
		}
		if applied == 0 {
			t.Errorf("%s: no VarSet controller found", path)
		}
	}
}

// Ikemen GO varSetOlderSub (compiler_functions.go:2705-2735): v, fv, sysv,
// sysfv each pair with value.
func TestApplyController_VarSetSysvOldForm_SetsSysVar(t *testing.T) {
	ctx, err := applyVarSetParams(t, evaluator.Context{}, map[string]string{"sysv": "3", "value": "8"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.SysVars[3] != 8 || ctx.Vars[3] != 0 {
		t.Errorf("SysVars[3] = %d, Vars[3] = %d, want 8 and 0", ctx.SysVars[3], ctx.Vars[3])
	}
}

func TestApplyController_VarSetOldFloatForms_ReturnFloatVarUnsupported(t *testing.T) {
	for _, key := range []string{"fv", "sysfv"} {
		_, err := applyVarSetParams(t, evaluator.Context{}, map[string]string{key: "1", "value": "1"})
		if !errors.Is(err, ErrFloatVarUnsupported) {
			t.Errorf("%s: error = %v, want ErrFloatVarUnsupported", key, err)
		}
	}
}

func TestApplyController_VarSetTwoOldFormKeys_ReturnsOneAtATimeError(t *testing.T) {
	for _, pair := range [][2]string{{"v", "sysv"}, {"v", "fv"}, {"sysv", "sysfv"}} {
		_, err := applyVarSetParams(t, evaluator.Context{}, map[string]string{pair[0]: "1", pair[1]: "1", "value": "1"})
		if err == nil || !strings.Contains(err.Error(), "only set one variable at a time") {
			t.Errorf("%v: error = %v, want one-at-a-time error", pair, err)
		}
	}
	_, err := applyVarSetParams(t, evaluator.Context{}, map[string]string{"var(1)": "1", "sysv": "1", "value": "1"})
	if err == nil || !strings.Contains(err.Error(), "only set one variable at a time") {
		t.Errorf("var(1)+sysv: error = %v, want one-at-a-time error", err)
	}
}

func TestApplyController_VarSetNoVariable_MessageListsAcceptedForms(t *testing.T) {
	_, err := applyVarSetParams(t, evaluator.Context{}, map[string]string{"value": "1"})
	if err == nil || !strings.Contains(err.Error(), "sysv") || !strings.Contains(err.Error(), "fvar(N)") {
		t.Errorf("error = %v, want message listing the accepted forms", err)
	}
}

func TestApplyController_VarSetForms_DoNotAllocateWhenWarm(t *testing.T) {
	for name, params := range map[string]map[string]string{
		"var(N)":    {"var(3)": "1"},
		"v/value":   {"v": "3", "value": "1"},
		"sysv":      {"sysv": "2", "value": "1"},
		"sysvar(N)": {"sysvar(2)": "1"},
	} {
		ctrl := cns.Controller{Type: "VarSet", Parameters: params}
		var ctx evaluator.Context
		apply := func() {
			if _, err := ApplyController(ctrl, &ctx, func(int) bool { return true }); err != nil {
				t.Fatal(err)
			}
		}
		apply() // warm the evaluator parse cache
		if allocs := testing.AllocsPerRun(100, apply); allocs != 0 {
			t.Errorf("%s: AllocsPerRun = %v, want 0", name, allocs)
		}
	}
}
