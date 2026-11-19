package engine

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/match"
)

// Item 060: a .cns Statedef header field is applied only when written, as
// Ikemen GO's stateDef.Run does (bytecode.go:5444-5512, commit 54d4148a):
// an omitted ctrl or anim keeps the fighter's current one, a written velset
// applies. character v0.17.0 records the written fields (cns.StateDef.Written).

// cnsEntryProgram parses src (state 0 changes to state 20 on the first tick).
func cnsEntryProgram(t *testing.T, src string) FighterProgram {
	t.Helper()
	states, err := cns.Parse(strings.NewReader("[Statedef 0]\ntype = S\nctrl = 1\nanim = 5\n[State 0]\ntype = ChangeState\ntrigger1 = 1\nvalue = 20\n\n" + src))
	if err != nil {
		t.Fatal(err)
	}
	byNumber := map[int]cns.StateDef{}
	for _, s := range states {
		byNumber[s.Number] = s
	}
	return NewFighterProgram(byNumber, nil, cmd.CommandFile{})
}

func enterTwenty(t *testing.T, src string) FighterRuntime {
	t.Helper()
	prog := cnsEntryProgram(t, src)
	result, err := tickP1Only(t, prog, newP1Runtime(t, prog))
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Fighters[match.SideP1].Context.StateNo; got != 20 {
		t.Fatalf("StateNo = %d, want 20", got)
	}
	return result.Fighters[match.SideP1]
}

func TestTick_CnsHeaderOmittingCtrlAndAnim_KeepsControlAndAnimation(t *testing.T) {
	// common1.cns:113-116: the walk state writes neither.
	ctx := enterTwenty(t, "[Statedef 20]\ntype = S\nphysics = S\nsprpriority = 0\n").Context
	if !ctx.Ctrl || ctx.Anim != 5 {
		t.Errorf("Ctrl, Anim = %v, %d, want true, 5 (kept)", ctx.Ctrl, ctx.Anim)
	}
}

func TestTick_CnsHeaderWritingCtrlZeroAndAnimZero_AppliesThem(t *testing.T) {
	ctx := enterTwenty(t, "[Statedef 20]\ntype = S\nctrl = 0\nanim = 0\n").Context
	if ctx.Ctrl || ctx.Anim != 0 {
		t.Errorf("Ctrl, Anim = %v, %d, want false, 0 (written)", ctx.Ctrl, ctx.Anim)
	}
}

func TestTick_CnsHeaderVelSet_SetsTheVelocityFacingRelative(t *testing.T) {
	// bytecode.go:5493-5500: X facing-relative, Y as written (MUGEN's down
	// is positive, the engine's up is positive).
	ctx := enterTwenty(t, "[Statedef 20]\ntype = A\nphysics = N\nvelset = 2, -4\n").Context
	if ctx.Velocity.X != 2 || ctx.Velocity.Y != 4 {
		t.Errorf("Velocity = %v, %v, want 2, 4", ctx.Velocity.X, ctx.Velocity.Y)
	}
}

func TestTick_CnsHeaderVelSetWithOnlyX_LeavesYAlone(t *testing.T) {
	ctx := enterTwenty(t, "[Statedef 20]\ntype = S\nphysics = N\nvelset = 3\n").Context
	if ctx.Velocity.X != 3 || ctx.Velocity.Y != 0 {
		t.Errorf("Velocity = %v, %v, want 3, 0", ctx.Velocity.X, ctx.Velocity.Y)
	}
}

func TestTick_CnsHeaderVelSetNotAnExpression_ReturnsAnErrorNamingTheState(t *testing.T) {
	prog := cnsEntryProgram(t, "[Statedef 20]\ntype = S\nvelset = 2, )(\n")
	_, err := tickP1Only(t, prog, newP1Runtime(t, prog))
	if err == nil || !strings.Contains(err.Error(), "state 20") {
		t.Errorf("err = %v, want one naming state 20", err)
	}
}

// The WebAssembly build decodes a request's states into FighterProgram
// (cmd/wasm programRequest); "written" comes from character's JSON.
func TestTick_CnsHeaderDecodedFromJSON_KeepsControlWhenCtrlIsNotWritten(t *testing.T) {
	raw := `{"states": {
		"0": {"number": 0, "type": "S", "ctrl": true, "written": {"ctrl": "1"},
		      "controllers": [{"type": "ChangeState", "triggers": [], "parameters": {"value": "20"}}]},
		"20": {"number": 20, "type": "S", "ctrl": false, "written": {"sprpriority": "0"}}}}`
	var decoded FighterProgram
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatal(err)
	}
	prog := NewFighterProgram(decoded.States, nil, cmd.CommandFile{})
	result, err := tickP1Only(t, prog, newP1Runtime(t, prog))
	if err != nil {
		t.Fatal(err)
	}
	if ctx := result.Fighters[match.SideP1].Context; ctx.StateNo != 20 || !ctx.Ctrl {
		t.Errorf("StateNo, Ctrl = %d, %v, want 20, true", ctx.StateNo, ctx.Ctrl)
	}
}
