package engine

import (
	"reflect"
	"strings"
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
)

type freezeKind int

const (
	attackerPause freezeKind = iota
	defenderShake
)

func always(c cns.Controller) cns.Controller {
	c.Triggers = []string{"1"}
	return c
}

// tickFrozenP1 runs one Tick with P1 in state 0 (plus extra states), frozen
// for 3 ticks as an attacker in hit pause or as a defender in shake, with
// var(0) preset to 1. P2 is idle and far away.
func tickFrozenP1(t *testing.T, kind freezeKind, states map[int]cns.StateDef) TickResult {
	t.Helper()
	p1 := match.FighterState{Side: match.SideP1, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: 900}, Health: 1000}
	rt1, err := NewFighterRuntime(p1, states)
	if err != nil {
		t.Fatalf("NewFighterRuntime: %v", err)
	}
	rt2, err := NewFighterRuntime(p2, idleStates())
	if err != nil {
		t.Fatalf("NewFighterRuntime: %v", err)
	}
	rt1.Context.Vars[0] = 1
	rt1.Context.Time = 5
	rt1.Context.AnimElapsed = 7
	if kind == attackerPause {
		rt1.HitPause = 3
	} else {
		rt1.Context.GetHit.HitShakeTime = 3
	}
	state, err := match.NewMatchState(1, 1000, p1, p2)
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}
	progs := [2]FighterProgram{match.SideP1: {States: states}, match.SideP2: {States: idleStates()}}
	result, err := Tick(*state, progs, [2]FighterRuntime{rt1, rt2}, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Gravity: 0, Tick: 1, ComboWindow: 60})
	if err != nil {
		t.Fatalf("Tick returned an error: %v", err)
	}
	return result
}

func withFlag(c cns.Controller, flag string) cns.Controller {
	c.Parameters["ignorehitpause"] = flag
	return always(c)
}

func state0(controllers ...cns.Controller) map[int]cns.StateDef {
	return map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: controllers}}
}

var freezeKinds = map[string]freezeKind{"attacker hit pause": attackerPause, "defender shake": defenderShake}

func TestTick_IgnoreHitPauseController_RunsWhileFrozen(t *testing.T) {
	for name, kind := range freezeKinds {
		t.Run(name, func(t *testing.T) {
			states := state0(
				withFlag(ctrl("VarSet", "v", "1", "value", "42"), "1"),
				withFlag(ctrl("VelSet", "x", "3"), "1"),
			)
			result := tickFrozenP1(t, kind, states)
			rt := result.Fighters[match.SideP1]
			if rt.Context.Vars[1] != 42 {
				t.Errorf("var(1) = %d, want 42 (VarSet with ignorehitpause = 1 must run)", rt.Context.Vars[1])
			}
			if got := result.State.Fighters[match.SideP1].Velocity.X; got != 3 {
				t.Errorf("Velocity.X = %v, want 3 (VelSet with ignorehitpause = 1 must run)", got)
			}
		})
	}
}

func TestTick_ControllerWithoutIgnoreHitPause_StaysSkippedWhileFrozen(t *testing.T) {
	for name, kind := range freezeKinds {
		t.Run(name, func(t *testing.T) {
			states := state0(
				always(ctrl("VarSet", "v", "1", "value", "42")),
				withFlag(ctrl("VarSet", "v", "2", "value", "43"), "0"),
			)
			rt := tickFrozenP1(t, kind, states).Fighters[match.SideP1]
			if rt.Context.Vars[1] != 0 || rt.Context.Vars[2] != 0 {
				t.Errorf("var(1), var(2) = %d, %d, want 0, 0 (flag absent or 0 stays skipped)", rt.Context.Vars[1], rt.Context.Vars[2])
			}
		})
	}
}

func TestTick_UnflaggedControllerAfterAFlaggedOne_StaysSkippedWhileFrozen(t *testing.T) {
	states := state0(
		withFlag(ctrl("VarSet", "v", "1", "value", "1"), "1"),
		always(ctrl("VarSet", "v", "2", "value", "2")),
	)
	rt := tickFrozenP1(t, attackerPause, states).Fighters[match.SideP1]
	if rt.Context.Vars[1] != 1 || rt.Context.Vars[2] != 0 {
		t.Errorf("var(1), var(2) = %d, %d, want 1, 0", rt.Context.Vars[1], rt.Context.Vars[2])
	}
}

func TestTick_IgnoreHitPauseExpression_IsEvaluated(t *testing.T) {
	// var(0) is 1 in tickFrozenP1, var(3) is 0.
	states := state0(
		withFlag(ctrl("VarSet", "v", "1", "value", "10"), "var(0)"),
		withFlag(ctrl("VarSet", "v", "2", "value", "20"), "var(3)"),
	)
	rt := tickFrozenP1(t, attackerPause, states).Fighters[match.SideP1]
	if rt.Context.Vars[1] != 10 || rt.Context.Vars[2] != 0 {
		t.Errorf("var(1), var(2) = %d, %d, want 10, 0", rt.Context.Vars[1], rt.Context.Vars[2])
	}
}

func TestTick_InvalidIgnoreHitPauseExpression_FailsTheTick(t *testing.T) {
	states := state0(withFlag(ctrl("VarSet", "v", "1", "value", "1"), "nosuchtrigger +"))
	p1 := match.FighterState{Side: match.SideP1, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: 900}, Health: 1000}
	rt1, err := NewFighterRuntime(p1, states)
	if err != nil {
		t.Fatalf("NewFighterRuntime: %v", err)
	}
	rt1.HitPause = 3
	rt2, err := NewFighterRuntime(p2, idleStates())
	if err != nil {
		t.Fatalf("NewFighterRuntime: %v", err)
	}
	state, err := match.NewMatchState(1, 1000, p1, p2)
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}
	_, err = Tick(*state, [2]FighterProgram{{States: states}, {States: idleStates()}}, [2]FighterRuntime{rt1, rt2}, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Tick: 1, ComboWindow: 60})
	if err == nil || !strings.Contains(err.Error(), "ignorehitpause") {
		t.Fatalf("err = %v, want an error naming ignorehitpause", err)
	}
}

func TestTick_IgnoreHitPauseControllerWithFalseTrigger_DoesNotRunWhileFrozen(t *testing.T) {
	for name, kind := range freezeKinds {
		t.Run(name, func(t *testing.T) {
			c := ctrl("VarSet", "v", "1", "value", "42")
			c.Parameters["ignorehitpause"] = "1"
			c.Triggers = []string{"0"}
			rt := tickFrozenP1(t, kind, state0(c)).Fighters[match.SideP1]
			if rt.Context.Vars[1] != 0 {
				t.Errorf("var(1) = %d, want 0 (flagged controller whose trigger is false must not run)", rt.Context.Vars[1])
			}
		})
	}
}

func TestTick_SkippedControllerWithInvalidTrigger_DoesNotFailFrozenTick(t *testing.T) {
	for name, kind := range freezeKinds {
		t.Run(name, func(t *testing.T) {
			c := ctrl("VarSet", "v", "1", "value", "42")
			c.Triggers = []string{"nosuchtrigger +"}
			// tickFrozenP1 fails the test if Tick returns an error.
			rt := tickFrozenP1(t, kind, state0(c)).Fighters[match.SideP1]
			if rt.Context.Vars[1] != 0 {
				t.Errorf("var(1) = %d, want 0", rt.Context.Vars[1])
			}
		})
	}
}

func TestTickFighter_IgnoreHitPauseHitDef_StartsNoHitWhileFrozen(t *testing.T) {
	hitDef := withFlag(ctrl("HitDef", "damage", "10"), "1")
	states := state0(hitDef)
	prog := FighterProgram{States: states}
	fighter := match.FighterState{Side: match.SideP1, Health: 1000}
	build := func(hitPause int) FighterRuntime {
		rt, err := NewFighterRuntime(fighter, states)
		if err != nil {
			t.Fatalf("NewFighterRuntime: %v", err)
		}
		rt.HitPause = hitPause
		return rt
	}

	unfrozen, err := tickFighter(prog, build(0), fighter, evaluator.Opponent{}, input.TickInput{}, 1, testBounds(), 0, 1, 0)
	if err != nil {
		t.Fatalf("tickFighter (unfrozen): %v", err)
	}
	if !unfrozen.HasHitDef {
		t.Fatal("control case: the same HitDef must start a hit on an unfrozen tick")
	}

	frozen, err := tickFighter(prog, build(3), fighter, evaluator.Opponent{}, input.TickInput{}, 1, testBounds(), 0, 1, 0)
	if err != nil {
		t.Fatalf("tickFighter (frozen): %v", err)
	}
	if frozen.HasHitDef {
		t.Error("HasHitDef = true, want false: a HitDef must not start a hit during a freeze, even with ignorehitpause")
	}
}

func TestTick_FlaggedChangeStateInStateMinusOneWhileFrozen_EntersNewStateAndRunsItsFlaggedController(t *testing.T) {
	states := state0()
	states[-1] = cns.StateDef{Number: -1, Controllers: []cns.Controller{
		withFlag(ctrl("ChangeState", "value", "300"), "1"),
	}}
	states[300] = cns.StateDef{Number: 300, Type: cns.StateTypeStanding, Anim: 301, Controllers: []cns.Controller{
		withFlag(ctrl("VarSet", "v", "1", "value", "9"), "1"),
		always(ctrl("VarSet", "v", "2", "value", "8")),
	}}
	ctx := tickFrozenP1(t, attackerPause, states).Fighters[match.SideP1].Context
	// Ikemen GO changes the state number during the pause (stateChange1) but
	// defers the entry -- the StateDef's anim among it -- until the pause
	// ends (stateChange2, char.go:6943-6944).
	if ctx.StateNo != 300 || ctx.Anim != 0 || ctx.Time != 0 || !ctx.StateEntryPending {
		t.Errorf("StateNo, Anim, Time, pending = %d, %d, %d, %v, want 300, 0, 0, true", ctx.StateNo, ctx.Anim, ctx.Time, ctx.StateEntryPending)
	}
	if ctx.Vars[1] != 9 || ctx.Vars[2] != 0 {
		t.Errorf("var(1), var(2) = %d, %d, want 9, 0 (new state's flagged controller runs, unflagged one does not)", ctx.Vars[1], ctx.Vars[2])
	}
}

func TestTick_NegativeStateControllerWithIgnoreHitPause_RunsWhileFrozen(t *testing.T) {
	states := state0()
	states[-1] = cns.StateDef{Number: -1, Controllers: []cns.Controller{
		withFlag(ctrl("VarSet", "v", "1", "value", "5"), "1"),
		always(ctrl("VarSet", "v", "2", "value", "6")),
	}}
	rt := tickFrozenP1(t, attackerPause, states).Fighters[match.SideP1]
	if rt.Context.Vars[1] != 5 || rt.Context.Vars[2] != 0 {
		t.Errorf("var(1), var(2) = %d, %d, want 5, 0", rt.Context.Vars[1], rt.Context.Vars[2])
	}
}

func TestTick_ChangeStateWithIgnoreHitPause_ChangesStateWhileFrozen(t *testing.T) {
	states := state0(withFlag(ctrl("ChangeState", "value", "300"), "1"))
	states[300] = cns.StateDef{Number: 300, Type: cns.StateTypeStanding, Anim: 300}
	result := tickFrozenP1(t, defenderShake, states)
	ctx := result.Fighters[match.SideP1].Context
	if ctx.StateNo != 300 || ctx.Anim != 0 || ctx.Time != 0 || !ctx.StateEntryPending {
		t.Errorf("StateNo, Anim, Time, pending = %d, %d, %d, %v, want 300, 0 (entry deferred, char.go:6943-6944), 0, true", ctx.StateNo, ctx.Anim, ctx.Time, ctx.StateEntryPending)
	}
	if got := result.State.Fighters[match.SideP1].StateNo; got != 300 {
		t.Errorf("match StateNo = %d, want 300", got)
	}
}

func TestTick_IgnoreHitPausePlaySnd_IsReturnedWhileFrozen(t *testing.T) {
	states := state0(withFlag(ctrl("PlaySnd", "value", "2, 4"), "1"))
	result := tickFrozenP1(t, attackerPause, states)
	want := []evaluator.SoundEvent{{Group: 2, Sample: 4, Channel: -1}}
	if !reflect.DeepEqual(result.Sounds[match.SideP1], want) {
		t.Errorf("Sounds = %v, want %v", result.Sounds[match.SideP1], want)
	}
}

func TestTick_FrozenTickWithIgnoreHitPauseController_HoldsTimeAndCountsDown(t *testing.T) {
	states := state0(withFlag(ctrl("VarSet", "v", "1", "value", "1"), "1"))
	result := tickFrozenP1(t, attackerPause, states)
	rt := result.Fighters[match.SideP1]
	if rt.Context.Time != 5 || rt.Context.AnimElapsed != 7 {
		t.Errorf("Time, AnimTime = %d, %d, want 5, 7 (held)", rt.Context.Time, rt.Context.AnimElapsed)
	}
	if rt.HitPause != 2 {
		t.Errorf("HitPause = %d, want 2", rt.HitPause)
	}
}

func TestTick_FrozenTickWithIgnoreHitPauseController_ReportsUnsupportedAndAddsNoAllocation(t *testing.T) {
	states := state0(
		withFlag(ctrl("VarSet", "v", "1", "value", "1"), "1"),
		withFlag(ctrl("NoSuchController"), "1"),
		always(ctrl("AnotherUnknown")),
	)
	result := tickFrozenP1(t, attackerPause, states)
	if want := []string{"nosuchcontroller"}; !reflect.DeepEqual(result.Unsupported[match.SideP1], want) {
		t.Errorf("Unsupported = %v, want %v (only the flagged controller ran)", result.Unsupported[match.SideP1], want)
	}
}

// A controller that applies costs one allocation per tick (statemachine's
// applied-index list), frozen or not; the frozen path must add none beyond
// what the same controllers cost on an unfrozen tick.
func TestTick_FrozenTickWithIgnoreHitPauseController_AllocatesNoMoreThanAnUnfrozenTickRunningIt(t *testing.T) {
	allocs := func(frozen bool) float64 {
		c := ctrl("VarSet", "v", "1", "value", "1")
		if frozen {
			c = withFlag(c, "1")
		} else {
			c = always(c)
		}
		states := state0(c)
		p1 := match.FighterState{Side: match.SideP1, Health: 1000}
		p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: 900}, Health: 1000}
		rt1, _ := NewFighterRuntime(p1, states)
		rt2, _ := NewFighterRuntime(p2, idleStates())
		if frozen {
			rt1.HitPause = 1 << 30
		}
		state, _ := match.NewMatchState(1, 1000, p1, p2)
		progs := [2]FighterProgram{{States: states}, {States: idleStates()}}
		tick := 1
		return testing.AllocsPerRun(100, func() {
			if _, err := Tick(*state, progs, [2]FighterRuntime{rt1, rt2}, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Tick: tick, ComboWindow: 60}); err != nil {
				t.Fatalf("Tick: %v", err)
			}
			tick++
		})
	}
	frozen, unfrozen := allocs(true), allocs(false)
	if frozen > unfrozen {
		t.Errorf("frozen tick allocated %v times, unfrozen tick running the same controller %v", frozen, unfrozen)
	}
}
