package engine

import (
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
)

// Item 042, review round: a ChangeState re-enters the state even when it
// targets the current number, and entering a state clears the HitDef unless
// hitdefpersist. Ikemen GO commit 54d4148a: changeStateEx always runs
// stateChange1/stateChange2 and sb.init (char.go:6966-6994); stateDef.Run
// clears mctime unless movehitpersist and the HitDef unless hitdefpersist
// (bytecode.go:5456-5463).

func reentryProgram(states map[int]cns.StateDef) FighterProgram {
	anims := []air.Animation{
		{Number: 0, Frames: []air.Frame{{Group: 0, Image: 0, Time: -1}}},
		{Number: 200, Frames: []air.Frame{{Group: 200, Image: 0, Time: 2}, {Group: 200, Image: 1, Time: 2}, {Group: 200, Image: 2, Time: 2}}},
	}
	return NewFighterProgram(states, anims, cmd.CommandFile{})
}

// stepP1 runs tickFighter n times on fighter P1, threading its runtime and
// its updated fighter state as Tick does.
func stepP1(t *testing.T, prog FighterProgram, rt FighterRuntime, n int) FighterRuntime {
	t.Helper()
	for i := 0; i < n; i++ {
		out, err := tickFighter(prog, rt, rt.Context.FighterState, evaluator.Opponent{}, input.TickInput{}, i+1, 0, 1, 0)
		if err != nil {
			t.Fatalf("tickFighter: %v", err)
		}
		rt = out.Runtime
	}
	return rt
}

func TestTick_ChangeStateIntoTheCurrentState_RestartsItsAnimationAndClearsMoveContact(t *testing.T) {
	// State 200 sends itself back to 200 at Time = 3, like kfm.cmd's
	// x-cancel of the jab (ChangeState value = 200 from state 200).
	states := map[int]cns.StateDef{
		0:   {Number: 0, Type: cns.StateTypeStanding, Ctrl: true},
		200: {Number: 200, Type: cns.StateTypeStanding, MoveType: cns.MoveTypeAttack, Anim: 200, Controllers: []cns.Controller{changeTo("200", "Time = 3")}},
	}
	prog := reentryProgram(states)
	rt, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000, StateNo: 200})
	if err != nil {
		t.Fatal(err)
	}
	rt.Context.MoveContactTime = 1 // a contact already happened in this jab
	// Ticks 1-3 run at Time 0, 1, 2; tick 4 runs at Time 3 and re-enters.
	rt = stepP1(t, prog, rt, 4)
	ctx := rt.Context
	if ctx.StateNo != 200 || ctx.Time != 0 || ctx.AnimElapsed != 0 {
		t.Errorf("StateNo, Time, AnimElapsed = %d, %d, %d, want 200, 0, 0 (state re-entered, animation restarted)", ctx.StateNo, ctx.Time, ctx.AnimElapsed)
	}
	if ctx.MoveContactTime != 0 {
		t.Errorf("MoveContactTime = %d, want 0: re-entering clears it (no movehitpersist)", ctx.MoveContactTime)
	}
}

func TestTick_EnteringAStateWithoutHitDefPersist_ClearsHitDefAttr(t *testing.T) {
	sca := mustAttr(t, "S, NA")
	states := map[int]cns.StateDef{
		0:   {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: []cns.Controller{changeTo("210", "1")}},
		210: {Number: 210, Type: cns.StateTypeStanding, MoveType: cns.MoveTypeAttack},
	}
	prog := reentryProgram(states)
	rt, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000})
	if err != nil {
		t.Fatal(err)
	}
	rt.Context.HitDefAttr = sca // left by an earlier attack state's HitDef
	rt = stepP1(t, prog, rt, 1)
	if rt.Context.StateNo != 210 {
		t.Fatalf("StateNo = %d, want 210", rt.Context.StateNo)
	}
	if evalBool(t, "HitDefAttr = SC, NA", rt.Context) {
		t.Error("HitDefAttr = SC, NA is true in a new attack state whose HitDef has not fired, want false (bytecode.go:5460-5463)")
	}
}

func TestTick_EnteringAStateWithHitDefPersist_KeepsHitDefAttr(t *testing.T) {
	states := map[int]cns.StateDef{
		0:   {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: []cns.Controller{changeTo("210", "1")}},
		210: {Number: 210, Type: cns.StateTypeStanding, MoveType: cns.MoveTypeAttack, HitDefPersist: true},
	}
	prog := reentryProgram(states)
	rt, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000})
	if err != nil {
		t.Fatal(err)
	}
	rt.Context.HitDefAttr = mustAttr(t, "S, NA")
	rt = stepP1(t, prog, rt, 1)
	if !evalBool(t, "HitDefAttr = SC, NA", rt.Context) {
		t.Error("HitDefAttr = SC, NA is false after entering a hitdefpersist state, want true")
	}
}

func TestTick_ChangeStateDuringHitPause_DefersTheEntryUntilThePauseEnds(t *testing.T) {
	// An ignorehitpause ChangeState fires while the attacker is frozen:
	// the number changes at once, the entry (anim, contact reset) waits
	// for the first unfrozen tick (char.go:6943-6944, 12529).
	states := map[int]cns.StateDef{
		0:   {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: []cns.Controller{withFlag(changeTo("200", "1"), "1")}},
		200: {Number: 200, Type: cns.StateTypeStanding, MoveType: cns.MoveTypeAttack, Anim: 200},
	}
	prog := reentryProgram(states)
	rt, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000})
	if err != nil {
		t.Fatal(err)
	}
	rt.Context.MoveContactTime = 1
	rt.HitPause = 1
	rt = stepP1(t, prog, rt, 1) // frozen tick
	if rt.Context.StateNo != 200 || rt.Context.Anim != 0 || rt.Context.MoveContactTime != 1 {
		t.Errorf("frozen: StateNo, Anim, MoveContactTime = %d, %d, %d, want 200, 0, 1 (entry deferred)", rt.Context.StateNo, rt.Context.Anim, rt.Context.MoveContactTime)
	}
	rt = stepP1(t, prog, rt, 1) // first unfrozen tick: the entry applies
	if rt.Context.Anim != 200 || rt.Context.MoveContactTime != 0 || rt.Context.StateEntryPending {
		t.Errorf("after the pause: Anim, MoveContactTime, pending = %d, %d, %v, want 200, 0, false", rt.Context.Anim, rt.Context.MoveContactTime, rt.Context.StateEntryPending)
	}
}

func mustAttr(t *testing.T, text string) int32 {
	t.Helper()
	a, err := evaluator.ParseAttr(text, true)
	if err != nil {
		t.Fatal(err)
	}
	return a
}
