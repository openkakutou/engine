package engine

import (
	"reflect"
	"strings"
	"testing"

	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

func mustScript(t *testing.T, source string) zss.Script {
	t.Helper()
	script, err := zss.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatalf("zss.Parse: %v", err)
	}
	return script
}

// zssProgram is a program whose states all come from one .zss source.
func zssProgram(t *testing.T, source string) FighterProgram {
	t.Helper()
	return NewFighterProgram(nil, nil, cmd.CommandFile{}).WithScripts([]zss.Script{mustScript(t, source)})
}

// tickP1Only runs one Tick where P1 plays prog from runtime rt and P2 idles
// far away, returning the result and Tick's error.
func tickP1Only(t *testing.T, prog FighterProgram, rt FighterRuntime) (TickResult, error) {
	t.Helper()
	p1 := match.FighterState{Side: match.SideP1, Health: 1000, StateNo: rt.Context.StateNo}
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: 900}, Health: 1000}
	rt2, err := NewFighterRuntime(p2, idleStates())
	if err != nil {
		t.Fatalf("NewFighterRuntime: %v", err)
	}
	st, err := match.NewMatchState(1, 1000, p1, p2)
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}
	return Tick(*st,
		[2]FighterProgram{match.SideP1: prog, match.SideP2: {States: idleStates()}},
		[2]FighterRuntime{match.SideP1: rt, match.SideP2: rt2},
		[2]input.TickInput{},
		TickConfig{Bounds: testBounds(), Gravity: 0, Tick: 1, ComboWindow: 60})
}

func newP1Runtime(t *testing.T, prog FighterProgram) FighterRuntime {
	t.Helper()
	rt, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000})
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	return rt
}

func TestTick_ZssCurrentState_RunsItsBlock(t *testing.T) {
	prog := zssProgram(t, "[Statedef 0; type: S; physics: S;]\nvarSet{v: 1; value: 5}\nvelSet{x: 3}")
	result, err := tickP1Only(t, prog, newP1Runtime(t, prog))
	if err != nil {
		t.Fatalf("Tick returned an error: %v", err)
	}
	rt := result.Fighters[match.SideP1]
	if rt.Context.Vars[1] != 5 {
		t.Errorf("var(1) = %d, want 5", rt.Context.Vars[1])
	}
	// velSet 3, the move, then physics S's stand friction 0.85
	// (char.go:10121-10127).
	if got, want := result.State.Fighters[match.SideP1].Velocity.X, 3*float64(float32(0.85)); got != want {
		t.Errorf("Velocity.X = %v, want %v", got, want)
	}
	if rt.Context.Time != 1 {
		t.Errorf("Time = %d, want 1 (an unchanged state's time advances as for .cns)", rt.Context.Time)
	}
}

func TestNewRuntime_ZssStartState_TakesItsHeader(t *testing.T) {
	// Ikemen GO stateDef (src/compiler.go:7020): type, physics, anim, ctrl.
	prog := zssProgram(t, "[Statedef 0; type: C; physics: C; anim: 11; ctrl: 1;]\nnull{}")
	rt := newP1Runtime(t, prog)
	if rt.Context.StateType != 'C' || rt.Context.Physics != 'C' || rt.Context.Anim != 11 || !rt.Context.Ctrl {
		t.Errorf("StateType, Physics, Anim, Ctrl = %c, %c, %d, %v, want C, C, 11, true",
			rt.Context.StateType, rt.Context.Physics, rt.Context.Anim, rt.Context.Ctrl)
	}
}

func TestNewRuntime_StateInNeitherFormat_ReturnsError(t *testing.T) {
	prog := zssProgram(t, "[Statedef 5]\nnull{}")
	_, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000})
	if err == nil || !strings.Contains(err.Error(), "state 0") {
		t.Errorf("err = %v, want one naming state 0", err)
	}
}

func TestTick_CnsStateChangesToZssStateAndBack_BothEntered(t *testing.T) {
	// Ikemen GO changes state in one table (char.go changeState), whatever
	// the file format of the source and target states.
	cnsStates := map[int]cns.StateDef{
		0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: []cns.Controller{changeStateTo("Time = 0 && PrevStateNo != 7", "7")}},
	}
	script := mustScript(t, "[Statedef 7; type: C; physics: C; anim: 77; ctrl: 0;]\nif time = 1 {\nchangeState{value: 0}\n}")
	prog := NewFighterProgram(cnsStates, nil, cmd.CommandFile{}).WithScripts([]zss.Script{script})

	rt := newP1Runtime(t, prog)
	result, err := tickP1Only(t, prog, rt)
	if err != nil {
		t.Fatalf("tick 1: %v", err)
	}
	ctx := result.Fighters[match.SideP1].Context
	if ctx.StateNo != 7 || ctx.Anim != 77 || ctx.StateType != 'C' || ctx.Ctrl {
		t.Errorf("after tick 1: StateNo, Anim, StateType, Ctrl = %d, %d, %c, %v, want 7, 77, C, false (the .zss header is applied on entry)",
			ctx.StateNo, ctx.Anim, ctx.StateType, ctx.Ctrl)
	}

	result, err = tickP1Only(t, prog, result.Fighters[match.SideP1])
	if err != nil {
		t.Fatalf("tick 2: %v", err)
	}
	ctx = result.Fighters[match.SideP1].Context
	if ctx.StateNo != 0 || ctx.StateType != 'S' || !ctx.Ctrl {
		t.Errorf("after tick 2: StateNo, StateType, Ctrl = %d, %c, %v, want 0, S, true (back in the .cns state)", ctx.StateNo, ctx.StateType, ctx.Ctrl)
	}
}

func TestTick_ZssStateChangesToCnsOnlyState_Applies(t *testing.T) {
	cnsStates := map[int]cns.StateDef{
		0:  {Number: 0, Type: cns.StateTypeStanding, Ctrl: true},
		20: {Number: 20, Type: cns.StateTypeStanding, Physics: cns.PhysicsStanding, Anim: 20, Ctrl: true},
	}
	prog := NewFighterProgram(cnsStates, nil, cmd.CommandFile{}).
		WithScripts([]zss.Script{mustScript(t, "[Statedef 100]\nchangeState{value: 20}")})
	rt := newP1Runtime(t, prog)
	rt.Context.StateNo = 100
	result, err := tickP1Only(t, prog, rt)
	if err != nil {
		t.Fatalf("Tick returned an error: %v", err)
	}
	if got := result.Fighters[match.SideP1].Context; got.StateNo != 20 || got.Anim != 20 {
		t.Errorf("StateNo, Anim = %d, %d, want 20, 20", got.StateNo, got.Anim)
	}
}

func TestTick_ZssSpecialStateMinusOne_ChangesTheCurrentStateInTheSameTick(t *testing.T) {
	// Ikemen GO actionRun (char.go): -1 runs before the current state, and the
	// state it changes to runs in the same tick.
	prog := zssProgram(t, "[Statedef 0; type: S; physics: S;]\nvarSet{v: 1; value: 111}\n"+
		"[Statedef -1]\nchangeState{value: 20}\n"+
		"[Statedef 20; type: S; physics: S; anim: 20;]\nvarSet{v: 2; value: 222}")
	result, err := tickP1Only(t, prog, newP1Runtime(t, prog))
	if err != nil {
		t.Fatalf("Tick returned an error: %v", err)
	}
	ctx := result.Fighters[match.SideP1].Context
	if ctx.StateNo != 20 || ctx.Anim != 20 {
		t.Errorf("StateNo, Anim = %d, %d, want 20, 20", ctx.StateNo, ctx.Anim)
	}
	if ctx.Vars[1] != 0 || ctx.Vars[2] != 222 {
		t.Errorf("var(1), var(2) = %d, %d, want 0, 222 (state 20 ran this tick, state 0 did not)", ctx.Vars[1], ctx.Vars[2])
	}
}

func TestNewFighterProgramFromFiles_StateInBothFormats_FirstFileWins(t *testing.T) {
	cnsFile := StateFile{States: []cns.StateDef{
		{Number: 0, Ctrl: true},
		{Number: 5, Controllers: []cns.Controller{setVar("1", 1, 11)}},
	}}
	zssFile := StateFile{Script: ptr(mustScript(t, "[Statedef 5]\nvarSet{v: 2; value: 22}"))}

	for name, tc := range map[string]struct {
		files          []StateFile
		wantV1, wantV2 int
	}{
		"cns file first": {[]StateFile{cnsFile, zssFile}, 11, 0},
		"zss file first": {[]StateFile{zssFile, cnsFile}, 0, 22},
	} {
		t.Run(name, func(t *testing.T) {
			prog := NewFighterProgramFromFiles(tc.files, nil, cmd.CommandFile{})
			rt := newP1Runtime(t, prog)
			rt.Context.StateNo = 5
			result, err := tickP1Only(t, prog, rt)
			if err != nil {
				t.Fatalf("Tick returned an error: %v", err)
			}
			ctx := result.Fighters[match.SideP1].Context
			if ctx.Vars[1] != tc.wantV1 || ctx.Vars[2] != tc.wantV2 {
				t.Errorf("var(1), var(2) = %d, %d, want %d, %d", ctx.Vars[1], ctx.Vars[2], tc.wantV1, tc.wantV2)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestTick_ZssBlockWithParseError_FailsNamingTheBlockOnlyWhenEntered(t *testing.T) {
	prog := zssProgram(t, "[Statedef 0; type: S;]\nnull{}\n[Statedef 9]\nfor i = 0, 3 {\n}")
	rt := newP1Runtime(t, prog)
	if _, err := tickP1Only(t, prog, rt); err != nil {
		t.Fatalf("Tick on the good state returned %v, want nil (a bad block elsewhere is only an error when entered)", err)
	}
	rt.Context.StateNo = 9
	_, err := tickP1Only(t, prog, rt)
	if err == nil || !strings.Contains(err.Error(), "Statedef 9") {
		t.Errorf("err = %v, want one naming \"Statedef 9\"", err)
	}
}

func TestTick_ZssChangeStateToUndefinedState_EntersEmptyStateAndWarns(t *testing.T) {
	prog := zssProgram(t, "[Statedef 0]\nchangeState{value: 4242}")
	result, err := tickP1Only(t, prog, newP1Runtime(t, prog))
	if err != nil {
		t.Fatalf("Tick: %v, want none", err)
	}
	if got := result.Fighters[match.SideP1].Context.StateNo; got != 4242 {
		t.Errorf("StateNo = %d, want 4242", got)
	}
	want := []string{"changed to invalid state 4242 (from state 0)"}
	if !reflect.DeepEqual(result.Warnings[match.SideP1], want) {
		t.Errorf("Warnings = %q, want %q", result.Warnings[match.SideP1], want)
	}
}

func TestTick_CurrentStateInNeitherFormat_RunsAsAnEmptyState(t *testing.T) {
	prog := zssProgram(t, "[Statedef 0]\nnull{}")
	rt := newP1Runtime(t, prog)
	rt.Context.StateNo = 31
	result, err := tickP1Only(t, prog, rt)
	if err != nil {
		t.Fatalf("Tick: %v, want none", err)
	}
	if got := result.Fighters[match.SideP1].Context.StateNo; got != 31 {
		t.Errorf("StateNo = %d, want 31 kept", got)
	}
}

func TestTick_ZssSkippedCallsAndUnsupportedControllers_ReachTheResultPerSide(t *testing.T) {
	prog := zssProgram(t, "[Statedef 0]\ncall Nope();\nsomeNewController{value: 1}\nsomeNewController{value: 2}")
	result, err := tickP1Only(t, prog, newP1Runtime(t, prog))
	if err != nil {
		t.Fatalf("Tick returned an error: %v", err)
	}
	if got := result.SkippedCalls[match.SideP1]; len(got) != 1 || !strings.Contains(got[0], "nope") {
		t.Errorf("SkippedCalls[P1] = %q, want one entry naming the call to nope", got)
	}
	if got := result.Unsupported[match.SideP1]; len(got) != 1 || got[0] != "somenewcontroller" {
		t.Errorf("Unsupported[P1] = %q, want [somenewcontroller] once", got)
	}
	if result.SkippedCalls[match.SideP2] != nil || result.Unsupported[match.SideP2] != nil {
		t.Errorf("P2 SkippedCalls, Unsupported = %v, %v, want nil, nil", result.SkippedCalls[match.SideP2], result.Unsupported[match.SideP2])
	}
	// The next tick starts afresh: a state without them reports none.
	rt := result.Fighters[match.SideP1]
	prog2 := zssProgram(t, "[Statedef 0]\nnull{}")
	result, err = tickP1Only(t, prog2, rt)
	if err != nil {
		t.Fatalf("second Tick returned an error: %v", err)
	}
	if result.SkippedCalls[match.SideP1] != nil || result.Unsupported[match.SideP1] != nil {
		t.Errorf("second tick SkippedCalls, Unsupported = %v, %v, want nil, nil", result.SkippedCalls[match.SideP1], result.Unsupported[match.SideP1])
	}
}

// Ikemen GO StateBlock.Run (src/bytecode.go:5136-5260): in a hit pause only
// the ignoreHitPause blocks run; Time and physics are held (decision 018).
func TestTick_ZssIgnoreHitPauseBlock_RunsWhileFrozenAndTheRestDoesNot(t *testing.T) {
	prog := zssProgram(t, "[Statedef 0; type: S; physics: S;]\n"+
		"varSet{v: 1; value: 1}\n"+
		"if 1 {\n varSet{v: 2; value: 2}\n}\n"+
		"ignoreHitPause {\n varSet{v: 3; value: 3}\n}")
	for name, freeze := range map[string]func(*FighterRuntime){
		"attacker hit pause": func(rt *FighterRuntime) { rt.HitPause = 3 },
		"defender shake":     func(rt *FighterRuntime) { rt.Context.GetHit.HitShakeTime = 3 },
	} {
		t.Run(name, func(t *testing.T) {
			rt := newP1Runtime(t, prog)
			freeze(&rt)
			rt.Context.Time = 5
			result, err := tickP1Only(t, prog, rt)
			if err != nil {
				t.Fatalf("Tick returned an error: %v", err)
			}
			got := result.Fighters[match.SideP1].Context
			if got.Vars[1] != 0 || got.Vars[2] != 0 || got.Vars[3] != 3 {
				t.Errorf("var(1..3) = %d, %d, %d, want 0, 0, 3", got.Vars[1], got.Vars[2], got.Vars[3])
			}
			if got.Time != 5 {
				t.Errorf("Time = %d, want 5 (held during a hit pause)", got.Time)
			}
		})
	}
}

func TestTick_ZssSpecialStateDuringHitPause_RunsOnlyItsFlaggedBlocks(t *testing.T) {
	prog := zssProgram(t, "[Statedef 0]\nnull{}\n"+
		"[Statedef -2]\nvarSet{v: 1; value: 1}\nignoreHitPause {\n varSet{v: 2; value: 2}\n}")
	rt := newP1Runtime(t, prog)
	rt.HitPause = 2
	result, err := tickP1Only(t, prog, rt)
	if err != nil {
		t.Fatalf("Tick returned an error: %v", err)
	}
	if got := result.Fighters[match.SideP1].Context; got.Vars[1] != 0 || got.Vars[2] != 2 {
		t.Errorf("var(1), var(2) = %d, %d, want 0, 2", got.Vars[1], got.Vars[2])
	}
}

func TestTick_ZssFighter_WarmTickAddsNoAllocationBeyondInputRecognition(t *testing.T) {
	prog := zssProgram(t, "[Statedef 0; type: S; physics: S;]\n"+
		"let a = 2;\nif time = 0 { velSet{y: 0} }\nvarSet{v: 1; value: $a}\n"+
		"[Statedef -1]\nif 0 { changeState{value: 0} }")
	p1 := match.FighterState{Side: match.SideP1, Position: match.Position{X: -10}, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: 10}, Health: 1000}
	rt1, err := prog.NewRuntime(p1)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	rt2, err := prog.NewRuntime(p2)
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	st, err := match.NewMatchState(1, 1000, p1, p2)
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}
	programs := [2]FighterProgram{prog, prog}
	runtimes := [2]FighterRuntime{rt1, rt2}
	bounds := testBounds()
	tick := 1
	run := func() {
		cfg := TickConfig{Bounds: bounds, Tick: tick, ComboWindow: 60}
		if _, err := Tick(*st, programs, runtimes, [2]input.TickInput{}, cfg); err != nil {
			t.Fatalf("Tick returned an error: %v", err)
		}
		tick++
	}
	run() // warm the zssexec local stack and the evaluator caches
	// Same baseline as the .cns path (TestTick_TickConfigAddsNoAllocation...):
	// input.Step's own 2 allocations per fighter.
	if allocs := testing.AllocsPerRun(100, run); allocs != 4 {
		t.Errorf("Tick with .zss fighters allocated %v times per call, want 4 (input recognition only)", allocs)
	}
}

// zssKfmHitDef is kfm_hitdef.cns's [State 200, 1] HitDef written as .zss.
const zssKfmHitDef = "hitDef{attr: S, NA; damage: 23, 0; animtype: Light; pausetime: 8, 8; ground.type: High;\n" +
	"\tground.slidetime: 5; ground.hittime: 11; air.type: High; air.hittime: 15}"

// newZssHitScenario is newHitScenario with the attacker's state 200 written
// in .zss as body (its header kept), P1 starting in it.
func newZssHitScenario(t *testing.T, body string) *hitScenario {
	t.Helper()
	s := newHitScenario(t, nil)
	states := make(map[int]cns.StateDef)
	for n, def := range s.programs[match.SideP1].States {
		if n != 200 {
			states[n] = def
		}
	}
	script := mustScript(t, "[Statedef 200; type: S; movetype: A; physics: S; anim: 200; ctrl: 0;]\n"+body)
	prog := NewFighterProgram(states, s.programs[match.SideP1].Animations, cmd.CommandFile{}).WithScripts([]zss.Script{script})
	rt, err := prog.NewRuntime(s.state.Fighter(match.SideP1))
	if err != nil {
		t.Fatalf("NewRuntime: %v", err)
	}
	s.programs[match.SideP1] = prog
	s.runtimes[match.SideP1] = rt
	return s
}

func TestTick_ZssHitDef_LandsExactlyAsTheSameCnsHitDef(t *testing.T) {
	cnsRun := newHitScenario(t, nil)
	zssRun := newZssHitScenario(t, "varSet{v: 0; value: var(0) + 1}\nif time = 0 {\n\t"+zssKfmHitDef+"\n}")

	for tick := 1; tick <= 14; tick++ {
		cnsRun.step()
		zssRun.step()
		for _, side := range []match.Side{match.SideP1, match.SideP2} {
			c, z := cnsRun.ctx(side), zssRun.ctx(side)
			if c.Health != z.Health || c.StateNo != z.StateNo || c.GetHit != z.GetHit || c.Vars != z.Vars ||
				c.MoveContactTime != z.MoveContactTime || c.HitDefAttr != z.HitDefAttr || c.HasTarget != z.HasTarget ||
				cnsRun.runtimes[side].HitPause != zssRun.runtimes[side].HitPause {
				t.Fatalf("tick %d side %v: .zss run differs from .cns run:\n cns %+v hitpause %d\n zss %+v hitpause %d",
					tick, side, c, cnsRun.runtimes[side].HitPause, z, zssRun.runtimes[side].HitPause)
			}
		}
		if tick == 1 {
			// The anchor the comparison stands on: the hit lands on tick 1.
			if got := zssRun.state.Fighter(match.SideP2).Health; got != 1000-23 {
				t.Fatalf("tick 1: defender health = %d, want %d", got, 1000-23)
			}
			if got := zssRun.ctx(match.SideP2).StateNo; got != 5000 {
				t.Fatalf("tick 1: defender StateNo = %d, want 5000", got)
			}
			if got := zssRun.ctx(match.SideP1).HitDefAttr; got == 0 {
				t.Fatalf("tick 1: attacker HitDefAttr = 0, want the HitDef's S, NA")
			}
		}
	}
}

func TestTick_ZssHitDefReadingLocals_UsesTheirValues(t *testing.T) {
	s := newZssHitScenario(t, "let d = 40;\nlet p = 6;\nif time = 0 {\n\thitDef{attr: S, NA; damage: $d, 0; pausetime: $p, $p + 1}\n}")
	s.step()
	if got := s.state.Fighter(match.SideP2).Health; got != 1000-40 {
		t.Errorf("defender health = %d, want %d (damage $d = 40)", got, 1000-40)
	}
	if got := s.runtimes[match.SideP1].HitPause; got != 6 {
		t.Errorf("attacker HitPause = %d, want 6 (pausetime $p)", got)
	}
	if got := s.ctx(match.SideP2).GetHit.HitShakeTime; got != 7 {
		t.Errorf("defender HitShakeTime = %d, want 7 (pausetime $p + 1)", got)
	}
}

func TestTick_ZssHitDefInACalledFunction_Lands(t *testing.T) {
	s := newZssHitScenario(t, "if time = 0 {\n\tcall Attack(31);\n}\n\n[Function Attack(dmg)]\nhitDef{attr: S, NA; damage: $dmg; pausetime: 8, 8}")
	s.step()
	if got := s.state.Fighter(match.SideP2).Health; got != 1000-31 {
		t.Errorf("defender health = %d, want %d", got, 1000-31)
	}
	if got := s.ctx(match.SideP2).StateNo; got != 5000 {
		t.Errorf("defender StateNo = %d, want 5000", got)
	}
}

func TestTick_ZssIgnoreHitPauseHitDef_StartsNoHitWhileFrozenButLandsOnceUnfrozen(t *testing.T) {
	// Decision 021 point 3, as for .cns: a frozen tick starts no hit.
	body := "ignoreHitPause {\n\thitDef{attr: S, NA; damage: 50; pausetime: 8, 8}\n}"
	frozen := newZssHitScenario(t, body)
	frozen.runtimes[match.SideP1].HitPause = 3
	frozen.step()
	if got := frozen.state.Fighter(match.SideP2).Health; got != 1000 {
		t.Errorf("frozen attacker: defender health = %d, want 1000 (no hit while frozen)", got)
	}

	unfrozen := newZssHitScenario(t, body)
	unfrozen.step()
	if got := unfrozen.state.Fighter(match.SideP2).Health; got != 1000-50 {
		t.Errorf("unfrozen attacker: defender health = %d, want %d", got, 1000-50)
	}
}
