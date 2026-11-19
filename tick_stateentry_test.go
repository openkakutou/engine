package engine

import (
	"testing"

	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
)

// Item 055: round-start control and the Statedef header on entry, as Ikemen
// GO commit 54d4148a runs them. Round start: system.go:3364 sets ctrl as the
// intro ends. Entry: StateBytecode.init then stateDef.Run (bytecode.go
// 17032-17086, 5444-5512), whose every field but the persist flags is
// optional (compiler.go:7020-7130) and evaluated on entry.

func TestNewRuntime_StartStateWithoutCtrl_GivesControlAsTheRoundStarts(t *testing.T) {
	cnsProg := NewFighterProgram(map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding}}, nil, cmd.CommandFile{})
	zssProg := zssProgram(t, "[Statedef 0; type: S; physics: S;]\nnull{}")
	for name, prog := range map[string]FighterProgram{".cns": cnsProg, ".zss": zssProg} {
		if rt := newP1Runtime(t, prog); !rt.Context.Ctrl {
			t.Errorf("%s: Ctrl = false at round start, want true (Ikemen GO system.go:3364)", name)
		}
	}
	rt, err := NewFighterRuntime(match.FighterState{Side: match.SideP1, Health: 1000}, map[int]cns.StateDef{0: {Number: 0}})
	if err != nil {
		t.Fatal(err)
	}
	if !rt.Context.Ctrl {
		t.Error("NewFighterRuntime: Ctrl = false at round start, want true")
	}
}

func TestTick_ZssEnteringAStateWithoutCtrlOrAnim_KeepsThemAndTheAnimationRuns(t *testing.T) {
	prog := zssProgram(t, "[Statedef 0; type: S; anim: 5;]\nchangeState{value: 20}\n[Statedef 20; type: S;]\nnull{}")
	result, err := tickP1Only(t, prog, newP1Runtime(t, prog))
	if err != nil {
		t.Fatal(err)
	}
	ctx := result.Fighters[match.SideP1].Context
	if ctx.StateNo != 20 || !ctx.Ctrl || ctx.Anim != 5 || ctx.AnimElapsed != 1 {
		t.Errorf("StateNo, Ctrl, Anim, AnimElapsed = %d, %v, %d, %d, want 20, true, 5, 1 (no ctrl or anim in the header: both kept, the animation not restarted)",
			ctx.StateNo, ctx.Ctrl, ctx.Anim, ctx.AnimElapsed)
	}
}

func TestTick_ZssHeaderWrittenAsExpressions_IsEvaluatedOnEntry(t *testing.T) {
	prog := zssProgram(t, "[Statedef 0; type: S;]\nchangeState{value: 20}\n"+
		"[Statedef 20; type: S; physics: N; anim: 100 + prevStateNo; ctrl: prevStateNo != 0; poweradd: 10 * 2; velset: 2 + 1, -4;]\nnull{}")
	result, err := tickP1Only(t, prog, newP1Runtime(t, prog))
	if err != nil {
		t.Fatal(err)
	}
	ctx := result.Fighters[match.SideP1].Context
	fighter := result.State.Fighters[match.SideP1]
	if ctx.StateNo != 20 || ctx.Anim != 100 || ctx.Ctrl || fighter.Power != 20 {
		t.Errorf("StateNo, Anim, Ctrl, Power = %d, %d, %v, %d, want 20, 100, false, 20", ctx.StateNo, ctx.Anim, ctx.Ctrl, fighter.Power)
	}
	// velset is facing-relative X and Ikemen GO's Y down (the engine keeps Y up).
	if fighter.Velocity.X != 3 || fighter.Velocity.Y != 4 {
		t.Errorf("Velocity = %+v, want X 3, Y 4 (velset = 3, -4)", fighter.Velocity)
	}
}

func TestTick_CnsHeaderExpressionsAndPowerAdd_ApplyOnEntry(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: []cns.Controller{changeTo("20")}},
		20: {Number: 20, Type: cns.StateTypeStanding, PowerAdd: 15,
			HeaderExprs: map[string]string{"anim": "100 + prevstateno", "ctrl": "prevstateno != 0"}},
	}
	prog := NewFighterProgram(states, nil, cmd.CommandFile{})
	result, err := tickP1Only(t, prog, newP1Runtime(t, prog))
	if err != nil {
		t.Fatal(err)
	}
	ctx := result.Fighters[match.SideP1].Context
	if ctx.StateNo != 20 || ctx.Anim != 100 || ctx.Ctrl || result.State.Fighters[match.SideP1].Power != 15 {
		t.Errorf("StateNo, Anim, Ctrl, Power = %d, %d, %v, %d, want 20, 100, false, 15",
			ctx.StateNo, ctx.Anim, ctx.Ctrl, result.State.Fighters[match.SideP1].Power)
	}
}

func TestTick_ChangeStateCtrlAndAnim_ApplyBeforeAHeaderThatOmitsThem(t *testing.T) {
	// Ikemen GO changeStateEx (char.go:6976-6983): anim then ctrl, before
	// the new state's header runs.
	toWalk := cns.Controller{Type: "ChangeState", Parameters: map[string]string{"value": "20", "ctrl": "0", "anim": "7"}}
	states := map[int]cns.StateDef{
		0:  {Number: 0, Type: cns.StateTypeStanding, Controllers: []cns.Controller{toWalk}},
		20: {Number: 20, Type: cns.StateTypeStanding, HeaderExprs: map[string]string{"ctrl": "ctrl"}},
	}
	prog := NewFighterProgram(states, nil, cmd.CommandFile{})
	result, err := tickP1Only(t, prog, newP1Runtime(t, prog))
	if err != nil {
		t.Fatal(err)
	}
	ctx := result.Fighters[match.SideP1].Context
	if ctx.StateNo != 20 || ctx.Ctrl || ctx.Anim != 7 {
		t.Errorf("StateNo, Ctrl, Anim = %d, %v, %d, want 20, false, 7", ctx.StateNo, ctx.Ctrl, ctx.Anim)
	}
}

func TestTick_ZssFaceP2_TurnsTowardAnOpponentBehind(t *testing.T) {
	// stateDef.Run facep2 (bytecode.go:5487-5490) turns when shouldFaceP2,
	// the opponent being behind (char.go:6775-6800).
	prog := zssProgram(t, "[Statedef 0; type: S;]\nchangeState{value: 40}\n[Statedef 40; type: S; facep2: 1;]\nnull{}")
	p1 := match.FighterState{Side: match.SideP1, Health: 1000, Position: match.Position{X: 100}, Facing: match.FacingLeft}
	p2 := match.FighterState{Side: match.SideP2, Health: 1000, Position: match.Position{X: 300}, Facing: match.FacingLeft}
	rt1, err := prog.NewRuntime(p1)
	if err != nil {
		t.Fatal(err)
	}
	rt2, err := NewFighterRuntime(p2, idleStates())
	if err != nil {
		t.Fatal(err)
	}
	st, err := match.NewMatchState(1, 1000, p1, p2)
	if err != nil {
		t.Fatal(err)
	}
	result, err := Tick(*st, [2]FighterProgram{prog, {States: idleStates()}}, [2]FighterRuntime{rt1, rt2},
		[2]input.TickInput{}, TickConfig{Bounds: testBounds(), Tick: 1, ComboWindow: 60})
	if err != nil {
		t.Fatal(err)
	}
	if got := result.State.Fighters[match.SideP1].Facing; got != match.FacingRight {
		t.Errorf("Facing = %v, want FacingRight (facep2 with the opponent behind)", got)
	}
}
