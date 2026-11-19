package engine

import (
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
)

// Item 064: a ChangeState made by the current state enters the new state and
// runs it in the same tick, again for every ChangeState that state makes, up
// to MaxLoop = 2500 runs (Ikemen GO commit 54d4148a, changeStateEx
// char.go:6966-6994, MaxLoop bytecode.go:15). Time counts from 0 in that run
// and grows once at the end of the tick (char.go:12592). See decision 033.

// countRuns is a VarSet adding 1 to var(0) on every run.
func countRuns() cns.Controller {
	return recordVar("0", "var(0) + 1")
}

func TestTick_ChangeStateByTheCurrentState_RunsTheNewStateInTheSameTick(t *testing.T) {
	states := map[int]cns.StateDef{
		0:   standing(0, changeTo("200", "1")),
		200: {Number: 200, Type: cns.StateTypeStanding, Anim: 200, Controllers: []cns.Controller{recordVar("0", "Time + 7")}},
	}
	prog := reentryProgram(states)
	rt, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000})
	if err != nil {
		t.Fatal(err)
	}
	rt = stepP1(t, prog, rt, 1)
	ctx := rt.Context
	if ctx.StateNo != 200 || ctx.Vars[0] != 7 {
		t.Fatalf("StateNo, var(0) = %d, %d, want 200, 7 (state 200 ran at Time 0 in the change tick)", ctx.StateNo, ctx.Vars[0])
	}
	if ctx.Time != 1 || ctx.AnimElapsed != 1 {
		t.Errorf("Time, AnimElapsed = %d, %d, want 1, 1 (char.go:12592; the animation steps after collision, char.go:13055-13061)", ctx.Time, ctx.AnimElapsed)
	}
	rt = stepP1(t, prog, rt, 1)
	if rt.Context.Vars[0] != 8 {
		t.Errorf("next tick var(0) = %d, want 8 (state 200 runs at Time 1)", rt.Context.Vars[0])
	}
}

func TestTick_ChangeStateByTheCurrentState_UsesTheNewAnimationsFirstFrameThisTick(t *testing.T) {
	// updateCurFrame after actionRun commits the new animation's frame for
	// collision detection (char.go:12608-12610).
	states := map[int]cns.StateDef{
		0:   standing(0, changeTo("200", "1")),
		200: {Number: 200, Type: cns.StateTypeStanding, Anim: 200},
	}
	prog := reentryProgram(states)
	rt, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000})
	if err != nil {
		t.Fatal(err)
	}
	out, err := tickFighter(prog, rt, rt.Context.FighterState, evaluator.Opponent{}, input.TickInput{}, 1, testBounds(), 0, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if out.Frame.Group != 200 || out.Frame.Image != 0 {
		t.Errorf("Frame = %d,%d, want 200,0 (state 200's animation)", out.Frame.Group, out.Frame.Image)
	}
}

func TestTick_ChainedChangeStates_RunEachStateInTheSameTick(t *testing.T) {
	states := map[int]cns.StateDef{
		0:   standing(0, changeTo("200", "1")),
		200: standing(200, countRuns(), changeTo("210", "1")),
		210: standing(210, countRuns()),
	}
	prog := reentryProgram(states)
	rt, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000})
	if err != nil {
		t.Fatal(err)
	}
	rt = stepP1(t, prog, rt, 1)
	if rt.Context.StateNo != 210 || rt.Context.PrevStateNo != 200 || rt.Context.Vars[0] != 2 {
		t.Errorf("StateNo, PrevStateNo, var(0) = %d, %d, %d, want 210, 200, 2 (200 then 210 ran)", rt.Context.StateNo, rt.Context.PrevStateNo, rt.Context.Vars[0])
	}
}

func TestTick_EndlessChangeStateLoop_StopsAfterMaxLoopRuns(t *testing.T) {
	// 0 and 1 change into each other every run. State 0 runs once, then the
	// loop runs 2500 new states (1, 0, 1, ... 0); the 2500th one's
	// ChangeState is refused (stateChange1, char.go:6803-6807), so the
	// fighter stays in state 0, entered from 1.
	states := map[int]cns.StateDef{
		0: standing(0, countRuns(), changeTo("1", "1")),
		1: standing(1, countRuns(), changeTo("0", "1")),
	}
	prog := reentryProgram(states)
	rt, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000})
	if err != nil {
		t.Fatal(err)
	}
	rt = stepP1(t, prog, rt, 1)
	ctx := rt.Context
	if ctx.Vars[0] != 2501 {
		t.Errorf("var(0) = %d, want 2501 (state 0's run plus MaxLoop = 2500)", ctx.Vars[0])
	}
	if ctx.StateNo != 0 || ctx.PrevStateNo != 1 || ctx.Time != 1 {
		t.Errorf("StateNo, PrevStateNo, Time = %d, %d, %d, want 0, 1, 1 (the last ChangeState refused)", ctx.StateNo, ctx.PrevStateNo, ctx.Time)
	}
}

func TestTick_ChangeStateDuringHitPause_RunsTheNewStatesIgnoreHitPauseControllers(t *testing.T) {
	// In hit pause stateChange2 waits (char.go:6943) but the loop still runs
	// the new state's bytecode, where only ignorehitpause controllers pass
	// (bytecode.go:5117-5121).
	states := map[int]cns.StateDef{
		0:   standing(0, withFlag(changeTo("200", "1"), "1")),
		200: {Number: 200, Type: cns.StateTypeStanding, Anim: 200, Controllers: []cns.Controller{withFlag(recordVar("0", "5"), "1"), recordVar("1", "5")}},
	}
	prog := reentryProgram(states)
	rt, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000})
	if err != nil {
		t.Fatal(err)
	}
	rt.HitPause = 2
	rt = stepP1(t, prog, rt, 1)
	ctx := rt.Context
	if ctx.StateNo != 200 || ctx.Vars[0] != 5 || ctx.Vars[1] != 0 {
		t.Errorf("StateNo, var(0), var(1) = %d, %d, %d, want 200, 5, 0 (only the flagged controller ran)", ctx.StateNo, ctx.Vars[0], ctx.Vars[1])
	}
	if ctx.Anim != 0 || !ctx.StateEntryPending {
		t.Errorf("Anim, pending = %d, %v, want 0, true (the entry waits for the pause to end)", ctx.Anim, ctx.StateEntryPending)
	}
}

func TestTick_ChangeStateIntoAStateWithAHitDef_ReportsItsHitDefThisTick(t *testing.T) {
	hitDef := cns.Controller{Type: "HitDef", Triggers: []string{"1"}, Parameters: map[string]string{"attr": "S, NA", "damage": "10"}}
	states := map[int]cns.StateDef{
		0:   standing(0, changeTo("200", "1")),
		200: {Number: 200, Type: cns.StateTypeStanding, MoveType: cns.MoveTypeAttack, Controllers: []cns.Controller{hitDef}},
	}
	prog := reentryProgram(states)
	rt, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000})
	if err != nil {
		t.Fatal(err)
	}
	out, err := tickFighter(prog, rt, rt.Context.FighterState, evaluator.Opponent{}, input.TickInput{}, 1, testBounds(), 0, 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !out.HasHitDef || !evalBool(t, "HitDefAttr = S, NA", out.Runtime.Context) {
		t.Errorf("HasHitDef = %v, HitDefAttr = S, NA false: want state 200's HitDef active in the change tick", out.HasHitDef)
	}
}

// Landing: state 52's own Time = 0 VelSet/PosSet put the fighter on the
// ground in the landing tick (common1.cns:273-281), as Ikemen GO runs it at
// once (changeStateEx from actionRun, char.go:12584-12590).
func TestTick_Landing_State52WithoutGroundControllers_StaysWhereTheMoveLeftIt(t *testing.T) {
	prog := FighterProgram{States: landingStates()}
	got := p1After(t, prog, match.FighterState{Position: match.Position{X: 5, Y: 1}, Velocity: match.Velocity{X: 2, Y: -2}, Health: 1000}, 0.5)
	if got.StateNo != 52 || got.Position.Y != -1 || got.Velocity.Y != -2.5 {
		t.Errorf("StateNo, Y, Vel Y = %d, %v, %v, want 52, -1, -2.5 (the engine no longer grounds the fighter itself)", got.StateNo, got.Position.Y, got.Velocity.Y)
	}
}
