package engine

import (
	"reflect"
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/engine/round"
	"github.com/openkakutou/stage"
)

func idleStates() map[int]cns.StateDef {
	return map[int]cns.StateDef{
		0: {Number: 0, Type: cns.StateTypeStanding, Anim: 0, Ctrl: true},
	}
}

func testBounds() *stage.StageBoundaries {
	return &stage.StageBoundaries{Left: -1000, Right: 1000}
}

func TestTick_AdvancesPhysics_WhenNoStateChangeOccurs(t *testing.T) {
	states := idleStates()
	p1 := FighterProgram{States: states}
	p2 := FighterProgram{States: states}

	p1Fighter := match.FighterState{Side: match.SideP1, Position: match.Position{X: -10, Y: 5}, Health: 1000}
	p2Fighter := match.FighterState{Side: match.SideP2, Position: match.Position{X: 10}, Health: 1000}

	p1Runtime, err := NewFighterRuntime(p1Fighter, states)
	if err != nil {
		t.Fatalf("NewFighterRuntime(p1): %v", err)
	}
	p2Runtime, err := NewFighterRuntime(p2Fighter, states)
	if err != nil {
		t.Fatalf("NewFighterRuntime(p2): %v", err)
	}

	state, err := match.NewMatchState(1, 1000, p1Fighter, p2Fighter)
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}

	result, err := Tick(
		*state,
		[2]FighterProgram{match.SideP1: p1, match.SideP2: p2},
		[2]FighterRuntime{match.SideP1: p1Runtime, match.SideP2: p2Runtime},
		[2]input.TickInput{},
		TickConfig{Bounds: testBounds(), Gravity: 1.0, Tick: 1, ComboWindow: 60},
	)
	if err != nil {
		t.Fatalf("Tick returned an error: %v", err)
	}

	// Y starts at 5 (airborne, since grounded requires Y<=0), so gravity
	// applies: velocity goes from 0 to -1.0, then integrates into
	// position, landing back on the ground plane (Y clamped to 0) since
	// 5 - 1.0 would still be > 0 -- actually 5 - 1.0 = 4, still airborne.
	got := result.State.Fighter(match.SideP1).Position.Y
	if got != 4 {
		t.Errorf("P1 Position.Y = %v, want 4 (5 - gravity 1.0)", got)
	}
	if result.Round.Outcome != round.OutcomeNone {
		t.Errorf("Round.Outcome = %v, want OutcomeNone", result.Round.Outcome)
	}
}

func TestTick_StateTransition_ResetsAnimAndAnimTime_ToTheEnteredStatesOwn(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {
			Number: 0, Type: cns.StateTypeStanding, Anim: 0, Ctrl: true,
			Controllers: []cns.Controller{
				{
					Type:       "ChangeState",
					Triggers:   []string{`Command = "atk"`},
					Parameters: map[string]string{"value": "200"},
				},
			},
		},
		200: {Number: 200, Type: cns.StateTypeStanding, Anim: 200, Ctrl: false},
	}
	commands := cmd.CommandFile{
		Defaults: cmd.CommandDefaults{Time: 15, BufferTime: 1},
		Commands: []cmd.Command{{Name: "atk", Input: "a"}},
	}
	prog := FighterProgram{States: states, Commands: commands}
	idleProg := FighterProgram{States: idleStates()}

	p1Fighter := match.FighterState{Side: match.SideP1, Health: 1000}
	p2Fighter := match.FighterState{Side: match.SideP2, Health: 1000}

	p1Runtime, err := NewFighterRuntime(p1Fighter, states)
	if err != nil {
		t.Fatalf("NewFighterRuntime(p1): %v", err)
	}
	p2Runtime, err := NewFighterRuntime(p2Fighter, idleStates())
	if err != nil {
		t.Fatalf("NewFighterRuntime(p2): %v", err)
	}

	state, err := match.NewMatchState(1, 1000, p1Fighter, p2Fighter)
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}

	p1Input := input.TickInput{Buttons: map[string]bool{"a": true}}

	result, err := Tick(
		*state,
		[2]FighterProgram{match.SideP1: prog, match.SideP2: idleProg},
		[2]FighterRuntime{match.SideP1: p1Runtime, match.SideP2: p2Runtime},
		[2]input.TickInput{match.SideP1: p1Input},
		TickConfig{Bounds: testBounds(), Gravity: 0, Tick: 1, ComboWindow: 60},
	)
	if err != nil {
		t.Fatalf("Tick returned an error: %v", err)
	}

	p1Ctx := result.Fighters[match.SideP1].Context
	if p1Ctx.StateNo != 200 {
		t.Fatalf("StateNo = %d, want 200", p1Ctx.StateNo)
	}
	if p1Ctx.Anim != 200 {
		t.Errorf("Anim = %d, want 200 (the entered state's own anim)", p1Ctx.Anim)
	}
	if p1Ctx.AnimTime != 0 {
		t.Errorf("AnimTime = %d, want 0", p1Ctx.AnimTime)
	}
	if p1Ctx.Time != 0 {
		t.Errorf("Time = %d, want 0", p1Ctx.Time)
	}
	if p1Ctx.Ctrl {
		t.Errorf("Ctrl = true, want false (state 200's own declared ctrl)")
	}
}

func TestTick_AppliesActiveHitDef_ReducesDefenderHealthAndReportsKO(t *testing.T) {
	attackerStates := map[int]cns.StateDef{
		200: {
			Number: 200, Type: cns.StateTypeStanding, Anim: 200, Ctrl: false,
			Controllers: []cns.Controller{
				{
					Type:       "HitDef",
					Triggers:   []string{"Time = 0"},
					Parameters: map[string]string{"damage": "30"},
				},
			},
		},
	}
	defenderStates := idleStates()

	attacker := FighterProgram{
		States: attackerStates,
		Animations: []air.Animation{
			{Number: 200, Frames: []air.Frame{{Time: 100, Clsn1: []air.ClsnBox{{Left: -5, Top: -50, Right: 5, Bottom: 0}}}}},
		},
	}
	defender := FighterProgram{
		States: defenderStates,
		Animations: []air.Animation{
			{Number: 0, Frames: []air.Frame{{Time: 100, Clsn2: []air.ClsnBox{{Left: -5, Top: -50, Right: 5, Bottom: 0}}}}},
		},
	}

	p1Fighter := match.FighterState{Side: match.SideP1, StateNo: 200, Health: 1000}
	p2Fighter := match.FighterState{Side: match.SideP2, StateNo: 0, Health: 20}

	p1Runtime, err := NewFighterRuntime(p1Fighter, attackerStates)
	if err != nil {
		t.Fatalf("NewFighterRuntime(p1): %v", err)
	}
	p2Runtime, err := NewFighterRuntime(p2Fighter, defenderStates)
	if err != nil {
		t.Fatalf("NewFighterRuntime(p2): %v", err)
	}

	state, err := match.NewMatchState(1, 1000, p1Fighter, p2Fighter)
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}

	result, err := Tick(
		*state,
		[2]FighterProgram{match.SideP1: attacker, match.SideP2: defender},
		[2]FighterRuntime{match.SideP1: p1Runtime, match.SideP2: p2Runtime},
		[2]input.TickInput{},
		TickConfig{Bounds: testBounds(), Gravity: 0, Tick: 1, ComboWindow: 60},
	)
	if err != nil {
		t.Fatalf("Tick returned an error: %v", err)
	}

	if got := result.State.Fighter(match.SideP2).Health; got != 0 {
		t.Errorf("P2 health = %d, want 0 (20 - 30, floored)", got)
	}
	if result.Round.Outcome != round.OutcomeKO {
		t.Fatalf("Round.Outcome = %v, want OutcomeKO", result.Round.Outcome)
	}
	if result.Round.Winner != match.SideP1 {
		t.Errorf("Round.Winner = %v, want SideP1", result.Round.Winner)
	}
}

func TestCurrentFrame_AdvancesThroughFramesAndLoopsAtLoopStart(t *testing.T) {
	anim := air.Animation{
		Number: 0,
		Frames: []air.Frame{
			{Time: 2, Image: 0}, // ticks 0-1
			{Time: 3, Image: 1}, // ticks 2-4
			{Time: 1, Image: 2}, // tick 5
		},
		LoopStart: 1, // loop back to frame index 1 (Image 1), not 0
	}

	cases := []struct {
		animTime  int
		wantImage int
	}{
		{0, 0},  // first frame
		{1, 0},  // still first frame
		{2, 1},  // second frame begins
		{4, 1},  // still second frame
		{5, 2},  // third frame
		{6, 1},  // wrapped to LoopStart (index 1), not index 0
		{9, 2},  // one loop pass (Time 3+1=4) after the wrap, into the third frame again
		{10, 1}, // a full loop pass (4 ticks) after the wrap, back at LoopStart's own frame
	}
	for _, c := range cases {
		got := currentFrame(anim, c.animTime)
		if got.Image != c.wantImage {
			t.Errorf("currentFrame(animTime=%d).Image = %d, want %d", c.animTime, got.Image, c.wantImage)
		}
	}
}

func TestCurrentFrame_ReturnsZeroFrame_WhenAnimationHasNoFrames(t *testing.T) {
	got := currentFrame(air.Animation{}, 5)
	if !reflect.DeepEqual(got, air.Frame{}) {
		t.Errorf("currentFrame on an empty animation = %+v, want zero Frame", got)
	}
}

func TestTick_TickConfigAddsNoAllocationBeyondInputRecognition(t *testing.T) {
	states := idleStates()
	p1 := FighterProgram{States: states}
	p2 := FighterProgram{States: states}

	p1Fighter := match.FighterState{Side: match.SideP1, Position: match.Position{X: -10}, Health: 1000}
	p2Fighter := match.FighterState{Side: match.SideP2, Position: match.Position{X: 10}, Health: 1000}

	p1Runtime, err := NewFighterRuntime(p1Fighter, states)
	if err != nil {
		t.Fatalf("NewFighterRuntime(p1): %v", err)
	}
	p2Runtime, err := NewFighterRuntime(p2Fighter, states)
	if err != nil {
		t.Fatalf("NewFighterRuntime(p2): %v", err)
	}

	state, err := match.NewMatchState(1, 1000, p1Fighter, p2Fighter)
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}

	programs := [2]FighterProgram{match.SideP1: p1, match.SideP2: p2}
	runtimes := [2]FighterRuntime{match.SideP1: p1Runtime, match.SideP2: p2Runtime}
	inputs := [2]input.TickInput{}
	bounds := testBounds()

	tick := 1
	allocs := testing.AllocsPerRun(100, func() {
		cfg := TickConfig{Bounds: bounds, Gravity: 0, Tick: tick, ComboWindow: 60}
		if _, err := Tick(*state, programs, runtimes, inputs, cfg); err != nil {
			t.Fatalf("Tick returned an error: %v", err)
		}
		tick++
	})

	// input.Step allocates its per-command progress/active maps fresh every
	// call -- 2 allocations per fighter, pre-existing and out of this
	// item's own scope (tracked separately, see roadmap engine backlog).
	// TickConfig itself is passed by value, so it must add nothing on top
	// of that baseline; a regression above 4 here would mean cfg started
	// escaping to the heap, exactly what TickConfig's own doc comment
	// warns against.
	const wantAllocs = 4
	if allocs != wantAllocs {
		t.Errorf("Tick allocated %v times per call across repeated ticks, want %d (see TickConfig's own doc comment)", allocs, wantAllocs)
	}
}

func TestTick_ReturnsError_WhenAFightersCurrentStateIsNotInItsLoadedStates(t *testing.T) {
	states := idleStates()
	p1 := FighterProgram{States: states}
	p2 := FighterProgram{States: states}

	p1Fighter := match.FighterState{Side: match.SideP1, StateNo: 999, Health: 1000}
	p2Fighter := match.FighterState{Side: match.SideP2, Health: 1000}

	p2Runtime, err := NewFighterRuntime(p2Fighter, states)
	if err != nil {
		t.Fatalf("NewFighterRuntime(p2): %v", err)
	}

	state, err := match.NewMatchState(1, 1000, p1Fighter, p2Fighter)
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}

	// p1Runtime is built by hand (not via NewFighterRuntime, which would
	// itself reject StateNo 999) to exercise Tick's own error path when a
	// caller-supplied runtime references a state absent from the loaded
	// character.
	p1Runtime := FighterRuntime{}
	p1Runtime.Context.FighterState = p1Fighter

	_, err = Tick(
		*state,
		[2]FighterProgram{match.SideP1: p1, match.SideP2: p2},
		[2]FighterRuntime{match.SideP1: p1Runtime, match.SideP2: p2Runtime},
		[2]input.TickInput{},
		TickConfig{Bounds: testBounds(), Gravity: 0, Tick: 1, ComboWindow: 60},
	)
	if err == nil {
		t.Fatal("expected an error for a fighter whose current state is not loaded, got nil")
	}
}
