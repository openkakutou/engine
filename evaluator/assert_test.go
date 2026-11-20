package evaluator

import "testing"

// compiler_functions.go:120-300 lists every AssertSpecial flag; any other
// name is an error ("Invalid AssertSpecial flag").
func TestAssertFlags(t *testing.T) {
	var f AssertFlags
	for _, name := range []string{"NoWalk", "nojump", "skipwindisplay"} {
		if err := f.Set(name, true); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
	if !f.Has("nowalk") || !f.Has("NOJUMP") || !f.Has("skipwindisplay") || f.Has("nocrouch") {
		t.Errorf("flags = %v", f)
	}
	if err := f.Set("nowalk", false); err != nil || f.Has("nowalk") {
		t.Errorf("enabled 0 clears the flag, err %v", err)
	}
	if err := f.Set("flyaway", true); err == nil {
		t.Error("an unknown flag must be an error")
	}
}

// bytecode.go:2510-2515: with noailevel asserted, AILevel reads 0.
func TestEval_NoAILevelHidesTheLevel(t *testing.T) {
	ctx := fighterCtx()
	ctx.AILevel = 6
	if err := ctx.Asserted.Set("noailevel", true); err != nil {
		t.Fatal(err)
	}
	if got := mustEval(t, "AILevel + AILevelF", ctx); got.Number != 0 {
		t.Errorf("AILevel + AILevelF = %v, want 0 under noailevel", got.Number)
	}
}
