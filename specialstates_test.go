package engine

import (
	"strconv"
	"strings"
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

func changeStateTo(trigger, target string) cns.Controller {
	return cns.Controller{Type: "ChangeState", Triggers: []string{trigger}, Parameters: map[string]string{"value": target}}
}

func setVar(trigger string, index, value int) cns.Controller {
	return cns.Controller{
		Type:     "VarSet",
		Triggers: []string{trigger},
		Parameters: map[string]string{
			"v":     strconv.Itoa(index),
			"value": strconv.Itoa(value),
		},
	}
}

// tickP1 runs one tick where only P1 has the given states and presses "a".
func tickP1(t *testing.T, states map[int]cns.StateDef, pressA bool) (TickResult, error) {
	t.Helper()
	commands := cmd.CommandFile{
		Defaults: cmd.CommandDefaults{Time: 15, BufferTime: 1},
		Commands: []cmd.Command{{Name: "atk", Input: "a"}},
	}
	prog := FighterProgram{States: states, Commands: commands}
	idle := FighterProgram{States: idleStates()}
	p1 := match.FighterState{Side: match.SideP1, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Health: 1000}
	r1, err := NewFighterRuntime(p1, states)
	if err != nil {
		t.Fatalf("NewFighterRuntime: %v", err)
	}
	r2, err := NewFighterRuntime(p2, idleStates())
	if err != nil {
		t.Fatalf("NewFighterRuntime: %v", err)
	}
	st, err := match.NewMatchState(1, 1000, p1, p2)
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}
	in := input.TickInput{Buttons: map[string]bool{"a": pressA}}
	return Tick(*st,
		[2]FighterProgram{match.SideP1: prog, match.SideP2: idle},
		[2]FighterRuntime{match.SideP1: r1, match.SideP2: r2},
		[2]input.TickInput{match.SideP1: in},
		TickConfig{Bounds: testBounds(), Gravity: 0, Tick: 1, ComboWindow: 60})
}

func TestTick_CommandInState1_MovesFighterIntoSpecialMoveAndRunsItThisTick(t *testing.T) {
	states := map[int]cns.StateDef{
		-1:  {Number: -1, Controllers: []cns.Controller{changeStateTo(`Command = "atk"`, "200")}},
		0:   {Number: 0, Type: cns.StateTypeStanding, Ctrl: true},
		200: {Number: 200, Type: cns.StateTypeStanding, Anim: 200, Controllers: []cns.Controller{setVar("Time = 0", 0, 7)}},
	}
	res, err := tickP1(t, states, true)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	ctx := res.Fighters[match.SideP1].Context
	if ctx.StateNo != 200 {
		t.Fatalf("StateNo = %d, want 200", ctx.StateNo)
	}
	if ctx.Vars[0] != 7 {
		t.Errorf("var(0) = %d, want 7 (state 200's Time = 0 controller must run in the tick it is entered)", ctx.Vars[0])
	}
	if ctx.Anim != 200 {
		t.Errorf("Anim = %d, want 200", ctx.Anim)
	}
	if ctx.Time != 1 {
		t.Errorf("Time = %d, want 1 (entered at Time 0, advanced once at the end of the tick)", ctx.Time)
	}
}

func TestTick_SpecialStates_RunInOrderMinus3Minus2Minus1BeforeCurrentState(t *testing.T) {
	// Each state only fires once the previous one has left its mark, so the
	// vars can all be set only if the order is -3, -2, -1, current.
	states := map[int]cns.StateDef{
		-3: {Number: -3, Controllers: []cns.Controller{setVar("1", 0, 3)}},
		-2: {Number: -2, Controllers: []cns.Controller{setVar("var(0) = 3", 1, 2)}},
		-1: {Number: -1, Controllers: []cns.Controller{setVar("var(1) = 2", 2, 1)}},
		0:  {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: []cns.Controller{setVar("var(2) = 1", 3, 9)}},
	}
	res, err := tickP1(t, states, false)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	v := res.Fighters[match.SideP1].Context.Vars
	if v[0] != 3 || v[1] != 2 || v[2] != 1 || v[3] != 9 {
		t.Errorf("vars 0..3 = %v, want [3 2 1 9] (-3, then -2, then -1, then the current state)", v[:4])
	}
}

func TestTick_ChangeStateInMinus3_DoesNotStopMinus2AndMinus1(t *testing.T) {
	states := map[int]cns.StateDef{
		-3:  {Number: -3, Controllers: []cns.Controller{changeStateTo("1", "200")}},
		-2:  {Number: -2, Controllers: []cns.Controller{setVar("StateNo = 200", 0, 5)}},
		-1:  {Number: -1, Controllers: []cns.Controller{setVar("StateNo = 200", 1, 6)}},
		0:   {Number: 0, Type: cns.StateTypeStanding, Ctrl: true},
		200: {Number: 200, Type: cns.StateTypeStanding},
	}
	res, err := tickP1(t, states, false)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	ctx := res.Fighters[match.SideP1].Context
	if ctx.StateNo != 200 || ctx.Vars[0] != 5 || ctx.Vars[1] != 6 {
		t.Errorf("StateNo=%d var0=%d var1=%d, want 200 5 6", ctx.StateNo, ctx.Vars[0], ctx.Vars[1])
	}
}

func TestTick_ChangeStateInAState_StopsThatStatesRemainingControllersOnly(t *testing.T) {
	states := map[int]cns.StateDef{
		-1:  {Number: -1, Controllers: []cns.Controller{changeStateTo("1", "200"), setVar("1", 0, 4)}},
		0:   {Number: 0, Type: cns.StateTypeStanding, Ctrl: true},
		200: {Number: 200, Type: cns.StateTypeStanding},
	}
	res, err := tickP1(t, states, false)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if v := res.Fighters[match.SideP1].Context.Vars[0]; v != 0 {
		t.Errorf("var(0) = %d, want 0: a ChangeState stops the rest of -1 (Ikemen GO bytecode.go changeState.Run returns stop)", v)
	}
}

func TestTick_NoSpecialStates_BehavesAsBefore(t *testing.T) {
	res, err := tickP1(t, idleStates(), false)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	ctx := res.Fighters[match.SideP1].Context
	if ctx.StateNo != 0 || ctx.Time != 1 {
		t.Errorf("StateNo=%d Time=%d, want 0 1", ctx.StateNo, ctx.Time)
	}
}

func TestTick_ChangeStateInState1_ToMissingState_ReturnsErrorNamingState1(t *testing.T) {
	states := map[int]cns.StateDef{
		-1: {Number: -1, Controllers: []cns.Controller{changeStateTo("1", "999")}},
		0:  {Number: 0, Type: cns.StateTypeStanding, Ctrl: true},
	}
	_, err := tickP1(t, states, false)
	if err == nil || !strings.Contains(err.Error(), "state -1") {
		t.Fatalf("err = %v, want an error mentioning state -1", err)
	}
}

func TestTick_SoundInSpecialState_IsReported(t *testing.T) {
	states := map[int]cns.StateDef{
		-1: {Number: -1, Controllers: []cns.Controller{{Type: "PlaySnd", Triggers: []string{"1"}, Parameters: map[string]string{"value": "5, 1"}}}},
		0:  {Number: 0, Type: cns.StateTypeStanding, Ctrl: true},
	}
	res, err := tickP1(t, states, false)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(res.Sounds[match.SideP1]) != 1 {
		t.Errorf("sounds = %v, want exactly one event from state -1", res.Sounds[match.SideP1])
	}
}

func TestTickFighter_StateEnteredFromState1_UsesNewAnimationsFirstFrameThisTick(t *testing.T) {
	states := map[int]cns.StateDef{
		// stateno = 0: a ChangeState into the current state re-enters it and
		// restarts its animation (Ikemen GO changeStateEx), so it must fire once.
		-1:  {Number: -1, Controllers: []cns.Controller{changeStateTo("stateno = 0", "200")}},
		0:   {Number: 0, Type: cns.StateTypeStanding, Ctrl: true},
		200: {Number: 200, Type: cns.StateTypeStanding, Anim: 200},
	}
	anims := []air.Animation{
		{Number: 0, Frames: []air.Frame{{Group: 0, Image: 0, Time: -1}}},
		{Number: 200, Frames: []air.Frame{{Group: 200, Image: 0, Time: 1}, {Group: 200, Image: 1, Time: 3}}},
	}
	prog := NewFighterProgram(states, anims, cmd.CommandFile{})
	fighter := match.FighterState{Side: match.SideP1, Health: 1000}
	runtime, err := NewFighterRuntime(fighter, states)
	if err != nil {
		t.Fatalf("NewFighterRuntime: %v", err)
	}

	out, err := tickFighter(prog, runtime, fighter, evaluator.Opponent{}, input.TickInput{}, 1, testBounds(), 0, 1, 0)
	if err != nil {
		t.Fatalf("tickFighter: %v", err)
	}
	if out.Frame.Group != 200 || out.Frame.Image != 0 {
		t.Errorf("frame this tick = %d,%d, want 200,0 (the entered animation's first frame)", out.Frame.Group, out.Frame.Image)
	}
	next, err := tickFighter(prog, out.Runtime, out.Runtime.Context.FighterState, evaluator.Opponent{}, input.TickInput{}, 2, testBounds(), 0, 1, 0)
	if err != nil {
		t.Fatalf("tickFighter: %v", err)
	}
	if next.Frame.Group != 200 || next.Frame.Image != 1 {
		t.Errorf("frame next tick = %d,%d, want 200,1", next.Frame.Group, next.Frame.Image)
	}
}
