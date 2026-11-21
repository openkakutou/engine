package engine

import (
	"math"
	"reflect"
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// tickPair runs one Tick with each fighter in the given state of its program.
func tickPair(t *testing.T, progs [2]FighterProgram, fighters [2]match.FighterState, gravity float64) TickResult {
	t.Helper()
	fighters[match.SideP1].Side = match.SideP1
	fighters[match.SideP2].Side = match.SideP2
	var runtimes [2]FighterRuntime
	for side := range runtimes {
		rt, err := NewFighterRuntime(fighters[side], progs[side].States)
		if err != nil {
			t.Fatalf("NewFighterRuntime(side %d): %v", side, err)
		}
		yaccel(t, &rt, gravity)
		runtimes[side] = rt
	}
	state, err := match.NewMatchState(1, 1000, fighters[match.SideP1], fighters[match.SideP2])
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}
	result, err := Tick(*state, progs, runtimes, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Gravity: gravity, Tick: 1, ComboWindow: 60})
	if err != nil {
		t.Fatalf("Tick returned an error: %v", err)
	}
	return result
}

// tickWithP1Program runs one Tick with P1 in state 0 of prog, starting as
// p1, against an idle P2 far enough away to never touch it.
func tickWithP1Program(t *testing.T, prog FighterProgram, p1 match.FighterState, gravity float64) TickResult {
	t.Helper()
	p2 := match.FighterState{Position: match.Position{X: 900}, Health: 1000}
	return tickPair(t, [2]FighterProgram{match.SideP1: prog, match.SideP2: {States: idleStates()}}, [2]match.FighterState{match.SideP1: p1, match.SideP2: p2}, gravity)
}

// p1After is tickWithP1Program's P1 match.FighterState.
func p1After(t *testing.T, prog FighterProgram, p1 match.FighterState, gravity float64) match.FighterState {
	t.Helper()
	state := tickWithP1Program(t, prog, p1, gravity).State
	return state.Fighters[match.SideP1]
}

func stateWith(controllers ...cns.Controller) map[int]cns.StateDef {
	for i := range controllers {
		controllers[i].Triggers = []string{"1"}
	}
	return map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: controllers}}
}

func ctrl(typ string, kv ...string) cns.Controller {
	params := map[string]string{}
	for i := 0; i < len(kv); i += 2 {
		params[kv[i]] = kv[i+1]
	}
	return cns.Controller{Type: typ, Parameters: params}
}

func TestTick_VelSet_MovesFighterOnTheSameTick(t *testing.T) {
	prog := FighterProgram{States: stateWith(ctrl("VelSet", "x", "3"))}
	got := p1After(t, prog, match.FighterState{Position: match.Position{X: -10}, Health: 1000}, 0)
	if got.Position.X != -7 || got.Velocity.X != 3 {
		t.Errorf("Position.X, Velocity.X = %v, %v, want -7, 3 (-10 + VelSet 3)", got.Position.X, got.Velocity.X)
	}
}

func TestTick_VelSetFacingLeft_MovesTowardNegativeX(t *testing.T) {
	prog := FighterProgram{States: stateWith(ctrl("VelSet", "x", "3"))}
	// P2 stands at 900, in front of P1 (at 950, facing left): no auto-turn.
	got := p1After(t, prog, match.FighterState{Position: match.Position{X: 950}, Facing: match.FacingLeft, Health: 1000}, 0)
	if got.Position.X != 947 {
		t.Errorf("Position.X = %v, want 947", got.Position.X)
	}
}

func TestTick_PosAdd_MovesFighterOnTheSameTick(t *testing.T) {
	prog := FighterProgram{States: stateWith(ctrl("PosAdd", "x", "5", "y", "-4"))}
	got := p1After(t, prog, match.FighterState{Position: match.Position{X: 100}, Health: 1000}, 0)
	// y -4 is up 4; the fighter is then airborne and gravity (0) leaves it there.
	if got.Position.X != 105 || got.Position.Y != 4 {
		t.Errorf("Position = %+v, want {105 4}", got.Position)
	}
}

func TestTick_Turn_ChangesFacingInTheResult(t *testing.T) {
	prog := FighterProgram{States: stateWith(ctrl("Turn"))}
	result := tickWithP1Program(t, prog, match.FighterState{Health: 1000}, 0)
	if got := result.State.Fighters[match.SideP1].Facing; got != match.FacingLeft {
		t.Errorf("Facing = %v, want FacingLeft", got)
	}
	if got := result.Fighters[match.SideP1].Context.Facing; got != match.FacingLeft {
		t.Errorf("runtime Context.Facing = %v, want FacingLeft", got)
	}
}

func TestTick_LifeAdd_ChangesHealthInTheResult(t *testing.T) {
	prog := FighterProgram{States: stateWith(ctrl("LifeAdd", "value", "-40"))}
	got := p1After(t, prog, match.FighterState{Health: 1000}, 0).Health
	if got != 960 {
		t.Errorf("Health = %d, want 960", got)
	}
}

func TestTick_LifeSetOnKOdFighter_DoesNotRevive(t *testing.T) {
	prog := FighterProgram{States: stateWith(ctrl("LifeSet", "value", "500"))}
	got := p1After(t, prog, match.FighterState{Health: 0}, 0).Health
	if got != 0 {
		t.Errorf("Health = %d, want 0", got)
	}
}

// physicsState is state 0 with the given physics type and controllers.
func physicsState(physics cns.PhysicsType, controllers ...cns.Controller) map[int]cns.StateDef {
	states := stateWith(controllers...)
	def := states[0]
	def.Physics = physics
	states[0] = def
	return states
}

// Ikemen GO char.go posUpdate applies gravity automatically only for
// ss.physics == ST_A; elsewhere gravity comes from the Gravity controller.
// With yaccel 0.5 and a fighter at rest at Y = 10, posUpdate moves it first
// (by 0) then adds gravity (char.go:10102-10139): Velocity.Y = -0.5, Y = 10.
// A Gravity controller runs with the states, before the movement.

func TestTick_PhysicsA_Airborne_FallsByYAccelOnceWithoutGravityController(t *testing.T) {
	prog := FighterProgram{States: physicsState(cns.PhysicsAir)}
	got := p1After(t, prog, match.FighterState{Position: match.Position{Y: 10}, Health: 1000}, 0.5)
	if got.Velocity.Y != -0.5 || got.Position.Y != 10 {
		t.Errorf("Velocity.Y, Position.Y = %v, %v, want -0.5, 10", got.Velocity.Y, got.Position.Y)
	}
}

func TestTick_PhysicsA_WithGravityController_FallsByTwiceYAccel(t *testing.T) {
	prog := FighterProgram{States: physicsState(cns.PhysicsAir, ctrl("Gravity"))}
	got := p1After(t, prog, match.FighterState{Position: match.Position{Y: 10}, Health: 1000}, 0.5)
	// Both are asked for, as in Ikemen GO: -0.5 from the controller, integrated, then -0.5 from physics A.
	if got.Velocity.Y != -1 || got.Position.Y != 9.5 {
		t.Errorf("Velocity.Y, Position.Y = %v, %v, want -1, 9.5", got.Velocity.Y, got.Position.Y)
	}
}

func TestTick_PhysicsN_WithGravityController_FallsByYAccelOnce(t *testing.T) {
	prog := FighterProgram{States: physicsState(cns.PhysicsNone, ctrl("Gravity"))}
	got := p1After(t, prog, match.FighterState{Position: match.Position{Y: 10}, Health: 1000}, 0.5)
	if got.Velocity.Y != -0.5 || got.Position.Y != 9.5 {
		t.Errorf("Velocity.Y, Position.Y = %v, %v, want -0.5, 9.5", got.Velocity.Y, got.Position.Y)
	}
}

func TestTick_PhysicsN_WithoutGravityController_KeepsVerticalVelocity(t *testing.T) {
	prog := FighterProgram{States: physicsState(cns.PhysicsNone)}
	got := p1After(t, prog, match.FighterState{Position: match.Position{Y: 10}, Velocity: match.Velocity{Y: 2}, Health: 1000}, 0.5)
	if got.Velocity.Y != 2 || got.Position.Y != 12 {
		t.Errorf("Velocity.Y, Position.Y = %v, %v, want 2, 12", got.Velocity.Y, got.Position.Y)
	}
}

func TestTick_OmittedPhysics_IsNone_SoNoAutomaticGravity(t *testing.T) {
	prog := FighterProgram{States: physicsState("")}
	got := p1After(t, prog, match.FighterState{Position: match.Position{Y: 10}, Health: 1000}, 0.5)
	if got.Velocity.Y != 0 || got.Position.Y != 10 {
		t.Errorf("Velocity.Y, Position.Y = %v, %v, want 0, 10 (Ikemen newStateBytecode: physics N)", got.Velocity.Y, got.Position.Y)
	}
}

func TestTick_StateTypeSetPhysicsA_TurnsAutomaticGravityOnForTheSameTick(t *testing.T) {
	prog := FighterProgram{States: physicsState(cns.PhysicsNone, ctrl("StateTypeSet", "physics", "A"))}
	got := p1After(t, prog, match.FighterState{Position: match.Position{Y: 10}, Health: 1000}, 0.5)
	if got.Velocity.Y != -0.5 || got.Position.Y != 10 {
		t.Errorf("Velocity.Y, Position.Y = %v, %v, want -0.5, 10", got.Velocity.Y, got.Position.Y)
	}
}

func TestEnterState_Physics_UnchangedForUAndUnknownLetters_NoneWhenOmitted(t *testing.T) {
	tests := []struct {
		name    string
		physics cns.PhysicsType
		want    byte
	}{
		{"air", cns.PhysicsAir, 'A'},
		{"lower case", "a", 'A'},
		{"omitted", "", 'N'},
		{"U keeps previous", "U", 'C'},
		{"unknown keeps previous", "X", 'C'},
	}
	for _, tc := range tests {
		ctx := evaluator.Context{Physics: 'C'}
		if _, err := enterState(&ctx, entryHeader{StateDef: cns.StateDef{Physics: tc.physics}}); err != nil {
			t.Fatal(err)
		}
		if ctx.Physics != tc.want {
			t.Errorf("%s: Physics = %c, want %c", tc.name, ctx.Physics, tc.want)
		}
	}
}

func TestTick_CtrlSetAndStateTypeSet_ChangeTheRuntimeContext(t *testing.T) {
	prog := FighterProgram{States: stateWith(
		ctrl("CtrlSet", "value", "0"),
		ctrl("StateTypeSet", "statetype", "A", "movetype", "A"),
	)}
	got := tickWithP1Program(t, prog, match.FighterState{Health: 1000}, 0).Fighters[match.SideP1].Context
	if got.Ctrl || got.StateType != 'A' || got.MoveType != 'A' {
		t.Errorf("Ctrl, StateType, MoveType = %v, %c, %c, want false, A, A", got.Ctrl, got.StateType, got.MoveType)
	}
}

func changeAnimProgram(extra ...string) FighterProgram {
	params := append([]string{"value", "300"}, extra...)
	return FighterProgram{
		States: stateWith(ctrl("ChangeAnim", params...)),
		// Elements last 5, 5 and 7 ticks: element 2 starts at AnimTime 5.
		Animations: []air.Animation{{Number: 300, Frames: []air.Frame{{Time: 5}, {Time: 5}, {Time: 7}}}},
	}
}

func TestTick_ChangeAnim_ChangesAnimAndRestartsItObservably(t *testing.T) {
	got := tickWithP1Program(t, changeAnimProgram(), match.FighterState{Health: 1000}, 0).Fighters[match.SideP1].Context
	// AnimTime restarts at 0 and the tick's own advance makes it 1, as Ikemen
	// GO runs anim.Action() after the controllers (char.go:13060).
	if got.Anim != 300 || got.AnimElapsed != 1 {
		t.Errorf("Anim, AnimTime = %d, %d, want 300, 1", got.Anim, got.AnimElapsed)
	}
	if got.AnimElem != 0 {
		t.Errorf("AnimElem = %d, want 0: the request is resolved within the tick", got.AnimElem)
	}
}

func TestTick_ChangeAnimElem_StartsAtThatElementPlusElemTime(t *testing.T) {
	got := tickWithP1Program(t, changeAnimProgram("elem", "2", "elemtime", "3"), match.FighterState{Health: 1000}, 0).Fighters[match.SideP1].Context
	// Element 2 starts at 5 ticks in; +3 into it = 8; the tick's advance = 9.
	if got.AnimElapsed != 9 {
		t.Errorf("AnimTime = %d, want 9", got.AnimElapsed)
	}
}

func TestTick_ChangeAnimElem_InvalidElemOrElemTime_FallBackLikeIkemen(t *testing.T) {
	tests := []struct {
		name  string
		extra []string
		want  int
	}{
		{"elem past the last", []string{"elem", "4", "elemtime", "2"}, 1},       // elem 1, time 0, advance 1
		{"elemtime past the frame", []string{"elem", "3", "elemtime", "7"}, 11}, // element 3 starts at 10, time 0, advance 1
		{"negative elemtime", []string{"elem", "2", "elemtime", "-1"}, 6},       // element 2 at 5, time 0, advance 1
	}
	for _, tc := range tests {
		got := tickWithP1Program(t, changeAnimProgram(tc.extra...), match.FighterState{Health: 1000}, 0).Fighters[match.SideP1].Context
		if got.AnimElapsed != tc.want {
			t.Errorf("%s: AnimTime = %d, want %d", tc.name, got.AnimElapsed, tc.want)
		}
	}
}

func TestTick_ChangeAnimElem_UnknownAnimation_RestartsAtZero(t *testing.T) {
	prog := changeAnimProgram("elem", "2")
	prog.Animations = nil
	got := tickWithP1Program(t, prog, match.FighterState{Health: 1000}, 0).Fighters[match.SideP1].Context
	if got.Anim != 300 || got.AnimElapsed != 1 {
		t.Errorf("Anim, AnimTime = %d, %d, want 300, 1", got.Anim, got.AnimElapsed)
	}
}

func TestTick_UnsupportedController_IsReportedByNameForItsFighterOnly(t *testing.T) {
	states := stateWith(ctrl("AppendToClipboard"), ctrl("appendtoclipboard"), ctrl("DisplayToClipboard"))
	states[-2] = cns.StateDef{Number: -2, Controllers: []cns.Controller{{Type: "AppendToClipboard", Triggers: []string{"1"}}, {Type: "SndPan", Triggers: []string{"1"}}}}
	result := tickWithP1Program(t, FighterProgram{States: states}, match.FighterState{Health: 1000}, 0)

	if want := []string{"appendtoclipboard", "sndpan", "displaytoclipboard"}; !reflect.DeepEqual(result.Unsupported[match.SideP1], want) {
		t.Errorf("Unsupported[SideP1] = %v, want %v (once per type across states -2 and 0)", result.Unsupported[match.SideP1], want)
	}
	if len(result.Unsupported[match.SideP2]) != 0 {
		t.Errorf("Unsupported[SideP2] = %v, want none", result.Unsupported[match.SideP2])
	}
}

func TestTick_OnlySupportedControllers_ReportsNoUnsupported(t *testing.T) {
	prog := FighterProgram{States: stateWith(ctrl("Null"), ctrl("Turn"))}
	result := tickWithP1Program(t, prog, match.FighterState{Health: 1000}, 0)
	if result.Unsupported[match.SideP1] != nil || result.Unsupported[match.SideP2] != nil {
		t.Errorf("Unsupported = %v, want none", result.Unsupported)
	}
}

func TestTick_P2Controllers_ReachP2sMatchState(t *testing.T) {
	p2States := stateWith(ctrl("VelSet", "x", "3"), ctrl("Turn"), ctrl("LifeAdd", "value", "-40"))
	result := tickPair(t,
		[2]FighterProgram{match.SideP1: {States: idleStates()}, match.SideP2: {States: p2States}},
		[2]match.FighterState{
			match.SideP1: {Position: match.Position{X: 900}, Facing: match.FacingLeft, Health: 1000},
			match.SideP2: {Position: match.Position{X: -900}, Health: 1000},
		}, 0)

	got := result.State.Fighters[match.SideP2]
	// P1 is in front of P2 (no auto-turn). VelSet 3 forward while facing
	// right: +3 world; Turn then flips the facing but not the world velocity.
	if got.Position.X != -897 || got.Velocity.X != 3 || got.Facing != match.FacingLeft || got.Health != 960 {
		t.Errorf("P2 = X %v, VelX %v, Facing %v, Health %d, want -897, 3, FacingLeft, 960", got.Position.X, got.Velocity.X, got.Facing, got.Health)
	}
	if p1 := result.State.Fighters[match.SideP1]; p1.Position.X != 900 || p1.Health != 1000 {
		t.Errorf("P1 = %+v, want untouched", p1)
	}
}

// hitProgram is an attacker state 0 whose HitDef (damage 30) is active at
// Time 0, together with extra controllers, over a frame with a +-5 hit box;
// idleHurtProgram is a defender with a +-5 hurt box.
func hitProgram(extra ...cns.Controller) FighterProgram {
	hit := cns.Controller{Type: "HitDef", Triggers: []string{"1"}, Parameters: map[string]string{"damage": "30", "attr": "S, NA"}}
	states := stateWith(append([]cns.Controller{hit}, extra...)...)
	def := states[0]
	def.MoveType = cns.MoveTypeAttack // a HitDef hits only while attacking (char.go:12984)
	states[0] = def
	box := []air.ClsnBox{{Left: -5, Top: -50, Right: 5, Bottom: 0}}
	return FighterProgram{States: states, Animations: []air.Animation{{Number: 0, Frames: []air.Frame{{Time: 100, Clsn1: box}}}}}
}

func hurtProgram(extra ...cns.Controller) FighterProgram {
	states := stateWith(extra...)
	box := []air.ClsnBox{{Left: -5, Top: -50, Right: 5, Bottom: 0}}
	return FighterProgram{States: states, Animations: []air.Animation{{Number: 0, Frames: []air.Frame{{Time: 100, Clsn2: box}}}}}
}

func TestTick_PosAddMovingAttackerIntoTheDefender_MakesTheHitLand(t *testing.T) {
	fighters := [2]match.FighterState{
		match.SideP1: {Position: match.Position{X: 0}, Health: 1000},
		match.SideP2: {Position: match.Position{X: 100}, Health: 1000},
	}
	missing := tickPair(t, [2]FighterProgram{hitProgram(), hurtProgram()}, fighters, 0)
	if got := missing.State.Fighters[match.SideP2].Health; got != 1000 {
		t.Fatalf("without the move P2 Health = %d, want 1000 (100 units apart)", got)
	}
	landing := tickPair(t, [2]FighterProgram{hitProgram(ctrl("PosAdd", "x", "100")), hurtProgram()}, fighters, 0)
	if got := landing.State.Fighters[match.SideP2].Health; got != 970 {
		t.Errorf("with PosAdd 100 P2 Health = %d, want 970 (the move happens before hit detection)", got)
	}
}

func TestTick_PosAddMovingAttackerOutOfTheDefender_MakesTheHitMiss(t *testing.T) {
	fighters := [2]match.FighterState{
		match.SideP1: {Position: match.Position{X: 100}, Health: 1000},
		match.SideP2: {Position: match.Position{X: 100}, Health: 1000},
	}
	landing := tickPair(t, [2]FighterProgram{hitProgram(), hurtProgram()}, fighters, 0)
	if got := landing.State.Fighters[match.SideP2].Health; got != 970 {
		t.Fatalf("without the move P2 Health = %d, want 970", got)
	}
	missing := tickPair(t, [2]FighterProgram{hitProgram(ctrl("PosAdd", "x", "50")), hurtProgram()}, fighters, 0)
	if got := missing.State.Fighters[match.SideP2].Health; got != 1000 {
		t.Errorf("with PosAdd 50 P2 Health = %d, want 1000 (the attacker has moved 50 away)", got)
	}
}

func TestTick_LifeAddAndLandingHitDefInTheSameTick_Sum(t *testing.T) {
	fighters := [2]match.FighterState{
		match.SideP1: {Health: 1000},
		match.SideP2: {Health: 1000},
	}
	result := tickPair(t, [2]FighterProgram{hitProgram(), hurtProgram(ctrl("LifeAdd", "value", "-50"))}, fighters, 0)
	if got := result.State.Fighters[match.SideP2].Health; got != 920 {
		t.Errorf("P2 Health = %d, want 920 (1000 - 50 LifeAdd - 30 hit)", got)
	}
}

func TestTick_ChangeAnimElem_AfterHeldOrZeroLengthFrames(t *testing.T) {
	// Frame durations -1 (held) and 0 contribute nothing to where element 3 starts.
	prog := changeAnimProgram("elem", "3", "elemtime", "2")
	prog.Animations = []air.Animation{{Number: 300, Frames: []air.Frame{{Time: -1}, {Time: 0}, {Time: 7}}}}
	got := tickWithP1Program(t, prog, match.FighterState{Health: 1000}, 0).Fighters[match.SideP1].Context
	// start 0 + elemtime 2 = 2, then the tick's advance: 3.
	if got.AnimElapsed != 3 {
		t.Errorf("AnimTime = %d, want 3", got.AnimElapsed)
	}
}

func TestTick_ChangeAnimInSpecialState_TakesEffect(t *testing.T) {
	states := stateWith()
	states[-2] = cns.StateDef{Number: -2, Controllers: []cns.Controller{
		{Type: "ChangeAnim", Triggers: []string{"1"}, Parameters: map[string]string{"value": "300", "elem": "2"}},
	}}
	prog := changeAnimProgram()
	prog.States = states
	got := tickWithP1Program(t, prog, match.FighterState{Health: 1000}, 0).Fighters[match.SideP1].Context
	// Element 2 starts at 5; the tick's advance: 6.
	if got.Anim != 300 || got.AnimElapsed != 6 {
		t.Errorf("Anim, AnimTime = %d, %d, want 300, 6", got.Anim, got.AnimElapsed)
	}
}

func TestTick_ChangeAnimThenChangeState_StateEntryWins(t *testing.T) {
	states := map[int]cns.StateDef{
		0: {Number: 0, Controllers: []cns.Controller{
			{Type: "ChangeAnim", Triggers: []string{"1"}, Parameters: map[string]string{"value": "300", "elem": "2"}},
			{Type: "ChangeState", Triggers: []string{"1"}, Parameters: map[string]string{"value": "200"}},
		}},
		200: {Number: 200, Anim: 210},
	}
	prog := changeAnimProgram()
	prog.States = states
	got := tickWithP1Program(t, prog, match.FighterState{Health: 1000}, 0).Fighters[match.SideP1].Context
	if got.StateNo != 200 || got.Anim != 210 || got.AnimElapsed != 1 || got.AnimElem != 0 {
		t.Errorf("StateNo, Anim, AnimTime, AnimElem = %d, %d, %d, %d, want 200, 210, 1, 0 (the new state ran in the change tick, then the animation stepped)", got.StateNo, got.Anim, got.AnimElapsed, got.AnimElem)
	}
}

// Allocation: Step's Result.Applied slice is the one allocation a firing
// controller has always cost, growing by doubling, so the baseline is a tick
// with as many PowerAdd controllers (a type that predates this item) firing.
// A tick where the new supported controllers fire, or an unsupported one
// does, must cost exactly that. The runtimes are threaded tick to tick so
// the per-fighter buffers are warm.
func allocsPerTick(t *testing.T, states map[int]cns.StateDef) float64 {
	t.Helper()
	p1 := match.FighterState{Side: match.SideP1, Position: match.Position{X: -10}, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: 10}, Health: 1000}
	idle := idleStates()
	programs := [2]FighterProgram{match.SideP1: {States: states}, match.SideP2: {States: idle}}
	var runtimes [2]FighterRuntime
	var err error
	if runtimes[match.SideP1], err = NewFighterRuntime(p1, states); err != nil {
		t.Fatal(err)
	}
	if runtimes[match.SideP2], err = NewFighterRuntime(p2, idle); err != nil {
		t.Fatal(err)
	}
	state, err := match.NewMatchState(1, 100000, p1, p2)
	if err != nil {
		t.Fatal(err)
	}
	st := *state
	tick := 1
	step := func() {
		result, err := Tick(st, programs, runtimes, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Gravity: 0.5, Tick: tick, ComboWindow: 60})
		if err != nil {
			t.Fatalf("Tick: %v", err)
		}
		st, runtimes = result.State, result.Fighters
		tick++
	}
	step() // warm the buffers and caches
	return testing.AllocsPerRun(100, step)
}

func powerAdds(n int) []cns.Controller {
	var out []cns.Controller
	for i := 0; i < n; i++ {
		out = append(out, ctrl("PowerAdd", "value", "1"))
	}
	return out
}

func TestTick_SupportedControllersFiring_AddNoAllocationOverPowerAdd(t *testing.T) {
	states := physicsState(cns.PhysicsAir,
		ctrl("VelSet", "x", "1", "y", "0"), ctrl("VelAdd", "x", "0.5"), ctrl("VelMul", "x", "0.5"),
		ctrl("PosSet", "y", "0"), ctrl("PosAdd", "x", "0"), ctrl("Gravity"), ctrl("Turn"), ctrl("Turn"),
		ctrl("CtrlSet", "value", "1"), ctrl("StateTypeSet", "statetype", "S", "movetype", "I", "physics", "A"),
		ctrl("LifeAdd", "value", "-1"), ctrl("LifeSet", "value", "900"), ctrl("Null"),
		ctrl("ChangeAnim", "value", "0", "elem", "1"),
	)
	want := allocsPerTick(t, physicsState(cns.PhysicsAir, powerAdds(14)...))
	if got := allocsPerTick(t, states); got != want {
		t.Errorf("Tick with 14 supported controllers firing allocated %v times, want %v (14 PowerAdd controllers)", got, want)
	}
}

func TestTick_UnsupportedControllerFiring_AddsNoAllocationOverPowerAdd(t *testing.T) {
	states := stateWith(ctrl("AppendToClipboard"), ctrl("appendtoclipboard"), ctrl("DisplayToClipboard"))
	want := allocsPerTick(t, stateWith(powerAdds(3)...))
	if got := allocsPerTick(t, states); got != want {
		t.Errorf("Tick with unsupported controllers firing allocated %v times, want %v (3 PowerAdd controllers)", got, want)
	}
}

// Ikemen GO char.go (hit resolution) chooses the reaction from the getter's
// live ss.stateType (e.g. `if getter.ss.stateType == ST_A { ghv.hittime =
// hd.air_hittime }`), which StateTypeSet and a U state type change at run
// time, not from the StateDef's declared type.

// reactionDefender is a defender whose state 0 has the given declared type
// and controllers, with get-hit states 5000 (ground) and 5020 (air).
func reactionDefender(declared cns.StateType, controllers ...cns.Controller) FighterProgram {
	prog := hurtProgram(controllers...)
	def := prog.States[0]
	def.Type = declared
	prog.States[0] = def
	prog.States[5000] = cns.StateDef{Number: 5000}
	prog.States[5020] = cns.StateDef{Number: 5020}
	return prog
}

func hitWithAirHitTime() FighterProgram {
	prog := hitProgram()
	prog.States[0].Controllers[0].Parameters["air.hittime"] = "17"
	prog.States[0].Controllers[0].Parameters["ground.hittime"] = "9"
	return prog
}

func TestTick_DefenderWhoStateTypeSetToAir_GetsTheAirReaction(t *testing.T) {
	defender := reactionDefender(cns.StateTypeStanding, ctrl("StateTypeSet", "statetype", "A"))
	result := tickPair(t, [2]FighterProgram{hitWithAirHitTime(), defender},
		[2]match.FighterState{match.SideP1: {Health: 1000}, match.SideP2: {Health: 1000}}, 0)
	got := result.Fighters[match.SideP2].Context
	if got.StateNo != 5020 || got.GetHit.HitTime != 17 {
		t.Errorf("StateNo, HitTime = %d, %d, want 5020, 17 (air reaction)", got.StateNo, got.GetHit.HitTime)
	}
}

func TestTick_DefenderInUnchangedTypeState_ReactsByTheInheritedAirType(t *testing.T) {
	// State 0 is declared type U: it keeps the type the fighter came in with.
	defender := reactionDefender(cns.StateType("U"))
	fighters := [2]match.FighterState{match.SideP1: {Health: 1000}, match.SideP2: {Health: 1000}}
	progs := [2]FighterProgram{hitWithAirHitTime(), defender}
	var runtimes [2]FighterRuntime
	for side := range runtimes {
		rt, err := NewFighterRuntime(fighters[side], progs[side].States)
		if err != nil {
			t.Fatal(err)
		}
		runtimes[side] = rt
	}
	runtimes[match.SideP2].Context.StateType = 'A' // arrived from an air state
	fighters[match.SideP1].Side, fighters[match.SideP2].Side = match.SideP1, match.SideP2
	state, err := match.NewMatchState(1, 1000, fighters[match.SideP1], fighters[match.SideP2])
	if err != nil {
		t.Fatal(err)
	}
	result, err := Tick(*state, progs, runtimes, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Tick: 1, ComboWindow: 60})
	if err != nil {
		t.Fatal(err)
	}
	got := result.Fighters[match.SideP2].Context
	if got.StateNo != 5020 || got.GetHit.HitTime != 17 {
		t.Errorf("StateNo, HitTime = %d, %d, want 5020, 17", got.StateNo, got.GetHit.HitTime)
	}
}

func TestTick_DefenderDeclaredAirButStateTypeSetToStanding_GetsTheGroundReaction(t *testing.T) {
	defender := reactionDefender(cns.StateTypeAir, ctrl("StateTypeSet", "statetype", "S"))
	result := tickPair(t, [2]FighterProgram{hitWithAirHitTime(), defender},
		[2]match.FighterState{match.SideP1: {Health: 1000}, match.SideP2: {Health: 1000}}, 0)
	got := result.Fighters[match.SideP2].Context
	if got.StateNo != 5000 || got.GetHit.HitTime != 9 {
		t.Errorf("StateNo, HitTime = %d, %d, want 5000, 9 (ground reaction)", got.StateNo, got.GetHit.HitTime)
	}
}

func TestEnterState_PersistentCounters_AreZeroed(t *testing.T) {
	ctx := evaluator.Context{}
	ctx.Persistent[0], ctx.Persistent[3] = 7, math.MaxInt32
	if _, err := enterState(&ctx, entryHeader{}); err != nil {
		t.Fatal(err)
	}
	if ctx.Persistent != [evaluator.MaxPersistent]int32{} {
		t.Errorf("Persistent = %v, want all zero (Ikemen GO char.go:6896-6903)", ctx.Persistent)
	}
}

// char.go:10102-10105: under PosFreeze the position stays, the velocity
// still takes the controllers.
func TestTick_PosFreezeKeepsThePositionThatTick(t *testing.T) {
	prog := FighterProgram{States: stateWith(ctrl("VelSet", "x", "3"), ctrl("PosFreeze"))}
	got := p1After(t, prog, match.FighterState{Position: match.Position{X: -10}, Health: 1000}, 0)
	if got.Position.X != -10 || got.Velocity.X != 3 {
		t.Errorf("Position.X, Velocity.X = %v, %v, want -10 kept, 3", got.Position.X, got.Velocity.X)
	}
}
