package engine

import (
	"reflect"
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

func tickBoth(t *testing.T, p1States, p2States map[int]cns.StateDef, seed int32, round int, p1Runtime, p2Runtime *FighterRuntime) TickResult {
	t.Helper()
	p1 := match.FighterState{Side: match.SideP1, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Health: 1000}
	var r1, r2 FighterRuntime
	var err error
	if p1Runtime != nil {
		p1.StateNo = p1Runtime.Context.StateNo
		r1 = *p1Runtime
	} else if r1, err = NewFighterRuntime(p1, p1States); err != nil {
		t.Fatal(err)
	}
	if p2Runtime != nil {
		r2 = *p2Runtime
	} else if r2, err = NewFighterRuntime(p2, p2States); err != nil {
		t.Fatal(err)
	}
	st, err := match.NewMatchState(round, 1000, p1, p2)
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}
	st.RandSeed = seed
	res, err := Tick(*st,
		[2]FighterProgram{{States: p1States}, {States: p2States}},
		[2]FighterRuntime{r1, r2},
		[2]input.TickInput{},
		TickConfig{Bounds: testBounds(), Tick: 1, ComboWindow: 60})
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	return res
}

func randomIntoVar() cns.Controller {
	return cns.Controller{Type: "VarSet", Triggers: []string{"1"}, Parameters: map[string]string{"v": "0", "value": "Random"}}
}

func TestTick_Random_BothFightersDrawFromOneSharedSequenceAndTheSeedIsCarriedInMatchState(t *testing.T) {
	states := map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: []cns.Controller{randomIntoVar()}}}
	// Seed 1 draws 0, 131 (independently computed from Ikemen GO's
	// generator): P1 acts first and takes the first draw.
	res := tickBoth(t, states, states, 1, 1, nil, nil)
	if got := res.Fighters[match.SideP1].Context.Vars[0]; got != 0 {
		t.Errorf("P1 Random = %d, want 0", got)
	}
	if got := res.Fighters[match.SideP2].Context.Vars[0]; got != 131 {
		t.Errorf("P2 Random = %d, want 131 (the second draw of the shared sequence)", got)
	}
	// The seed after two draws from 1 is 16807*16807 = 282475249.
	if res.State.RandSeed != 282475249 {
		t.Errorf("RandSeed after the tick = %d, want 282475249", res.State.RandSeed)
	}
}

func TestTick_Random_TwoMatchesFromTheSameSeedStayIdentical(t *testing.T) {
	states := map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: []cns.Controller{randomIntoVar()}}}
	a := tickBoth(t, states, states, 777, 1, nil, nil)
	b := tickBoth(t, states, states, 777, 1, nil, nil)
	if !reflect.DeepEqual(a.Fighters, b.Fighters) || a.State != b.State {
		t.Error("two ticks from the same seed gave different results")
	}
}

func TestTick_Random_ContextsDoNotKeepTheSeedBetweenTicks(t *testing.T) {
	// The seed lives in match state: a runtime carried from an earlier tick
	// must not override it.
	states := map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: []cns.Controller{randomIntoVar()}}}
	first := tickBoth(t, states, states, 1, 1, nil, nil)
	r1, r2 := first.Fighters[match.SideP1], first.Fighters[match.SideP2]
	second := tickBoth(t, states, states, 1, 1, &r1, &r2)
	if got := second.Fighters[match.SideP1].Context.Vars[0]; got != 0 {
		t.Errorf("P1 Random on a re-run from seed 1 = %d, want 0", got)
	}
}

func TestTick_StateTypeAndMoveType_FollowTheEnteredStateDefinition(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {Number: 0, Type: cns.StateTypeStanding, MoveType: cns.MoveTypeIdle, Ctrl: true,
			Controllers: []cns.Controller{{Type: "ChangeState", Triggers: []string{"StateType = S && MoveType = I"}, Parameters: map[string]string{"value": "600"}}}},
		600: {Number: 600, Type: cns.StateTypeAir, MoveType: cns.MoveTypeAttack},
	}
	res := tickBoth(t, states, states, 0, 1, nil, nil)
	ctx := res.Fighters[match.SideP1].Context
	if ctx.StateNo != 600 || ctx.StateType != 'A' || ctx.MoveType != 'A' {
		t.Errorf("StateNo=%d StateType=%c MoveType=%c, want 600 A A", ctx.StateNo, ctx.StateType, ctx.MoveType)
	}
	if ctx.PrevStateNo != 0 {
		t.Errorf("PrevStateNo = %d, want 0", ctx.PrevStateNo)
	}
}

func TestTick_StateDefWithTypeU_KeepsThePreviousStateType(t *testing.T) {
	states := map[int]cns.StateDef{
		0:   {Number: 0, Type: cns.StateTypeCrouching, Ctrl: true, Controllers: []cns.Controller{{Type: "ChangeState", Triggers: []string{"1"}, Parameters: map[string]string{"value": "700"}}}},
		700: {Number: 700, Type: "U"},
	}
	res := tickBoth(t, states, states, 0, 1, nil, nil)
	if got := res.Fighters[match.SideP1].Context.StateType; got != 'C' {
		t.Errorf("StateType after entering a type-U state = %c, want C (unchanged)", got)
	}
}

func TestTick_PrevStateNo_IsTheStateLeftByTheLastChange(t *testing.T) {
	states := map[int]cns.StateDef{
		0:   {Number: 0, Type: cns.StateTypeStanding, Ctrl: true},
		200: {Number: 200, Type: cns.StateTypeStanding, Controllers: []cns.Controller{{Type: "ChangeState", Triggers: []string{"PrevStateNo = 0"}, Parameters: map[string]string{"value": "210"}}}},
		210: {Number: 210, Type: cns.StateTypeStanding},
	}
	p1 := match.FighterState{Side: match.SideP1, Health: 1000, StateNo: 200}
	r, err := NewFighterRuntime(p1, states)
	if err != nil {
		t.Fatal(err)
	}
	r.Context.PrevStateNo = 0
	res := tickBoth(t, states, states, 0, 1, &r, nil)
	ctx := res.Fighters[match.SideP1].Context
	if ctx.StateNo != 210 || ctx.PrevStateNo != 200 {
		t.Errorf("StateNo=%d PrevStateNo=%d, want 210 and 200", ctx.StateNo, ctx.PrevStateNo)
	}
}

func TestTick_RoundNo_ReadsTheMatchRound(t *testing.T) {
	states := map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true,
		Controllers: []cns.Controller{{Type: "VarSet", Triggers: []string{"RoundNo = 3"}, Parameters: map[string]string{"v": "1", "value": "42"}}}}}
	res := tickBoth(t, states, states, 0, 3, nil, nil)
	if got := res.Fighters[match.SideP1].Context.Vars[1]; got != 42 {
		t.Errorf("var(1) = %d, want 42 (RoundNo = 3 should hold in round 3)", got)
	}
}

func TestTick_PositionTriggers_ReadTheLiveFighterPosition(t *testing.T) {
	states := map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true,
		Controllers: []cns.Controller{{Type: "VarSet", Triggers: []string{"Pos X = 0 && Life = 1000 && Facing = 1"}, Parameters: map[string]string{"v": "2", "value": "9"}}}}}
	res := tickBoth(t, states, states, 0, 1, nil, nil)
	if got := res.Fighters[match.SideP1].Context.Vars[2]; got != 9 {
		t.Errorf("var(2) = %d, want 9", got)
	}
}

func TestTick_AnimElem_FiresOnTheFirstTickOfThatElementOfTheFightersAnimation(t *testing.T) {
	// Animation 0 (state 0's own): elements of 2 ticks each, so element 2
	// starts 2 ticks in, the third tick (the elapsed counter is 0 on the first).
	// AnimTime there is curtime - totaltime = 2 - 6 = -4 (Ikemen GO anim.go:617).
	anims := []air.Animation{{Number: 0, Frames: []air.Frame{{Time: 2}, {Time: 2}, {Time: 2}}}}
	states := map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: []cns.Controller{
		{Type: "VarSet", Triggers: []string{"AnimElem = 2"}, Parameters: map[string]string{"v": "0", "value": "AnimTime + 100"}},
		{Type: "VarSet", Triggers: []string{"SelfAnimExist(0)", "!SelfAnimExist(1)"}, Parameters: map[string]string{"v": "1", "value": "7"}},
	}}}
	prog := NewFighterProgram(states, anims, cmd.CommandFile{})
	p1 := match.FighterState{Side: match.SideP1, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Health: 1000}
	rt1, err := prog.NewRuntime(p1)
	if err != nil {
		t.Fatal(err)
	}
	rt2, err := prog.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	st, err := match.NewMatchState(1, 1000, p1, p2)
	if err != nil {
		t.Fatal(err)
	}
	runtimes := [2]FighterRuntime{rt1, rt2}
	for tick := 1; tick <= 4; tick++ {
		res, err := Tick(*st, [2]FighterProgram{prog, prog}, runtimes, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Tick: tick, ComboWindow: 60})
		if err != nil {
			t.Fatalf("Tick %d: %v", tick, err)
		}
		st, runtimes = &res.State, res.Fighters
		want := 0
		if tick >= 3 {
			want = 96
		}
		if got := runtimes[match.SideP1].Context.Vars[0]; got != want {
			t.Errorf("after tick %d Vars[0] = %d, want %d", tick, got, want)
		}
		if got := runtimes[match.SideP1].Context.Vars[1]; got != 7 {
			t.Errorf("after tick %d SelfAnimExist(0) && !SelfAnimExist(1) left Vars[1] = %d, want 7", tick, got)
		}
	}
}
