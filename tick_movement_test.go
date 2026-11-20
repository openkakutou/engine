package engine

import (
	"strings"
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// tickOne runs one tick of a fighter in a state of the given physics with
// the given constants and starting velocity, and returns it.
func tickOne(t *testing.T, physics cns.PhysicsType, consts string, vel match.Velocity) match.FighterState {
	t.Helper()
	c, err := evaluator.ParseConstants(strings.NewReader(consts))
	if err != nil {
		t.Fatal(err)
	}
	states := map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, Physics: physics, Ctrl: true}}
	prog := FighterProgram{States: states, Constants: c}
	f1 := match.FighterState{Side: match.SideP1, Position: match.Position{X: -10, Y: 50}, Velocity: vel, Health: 1000}
	f2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: 300}, Health: 1000}
	r1, err := prog.NewRuntime(f1)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := prog.NewRuntime(f2)
	if err != nil {
		t.Fatal(err)
	}
	state, err := match.NewMatchState(1, 1000, f1, f2)
	if err != nil {
		t.Fatal(err)
	}
	res, err := Tick(*state, [2]FighterProgram{prog, prog}, [2]FighterRuntime{r1, r2}, [2]input.TickInput{},
		TickConfig{Bounds: testBounds(), Gravity: 9, Tick: 1, ComboWindow: 60})
	if err != nil {
		t.Fatal(err)
	}
	return res.State.Fighter(match.SideP1)
}

// char.go:9963-9965 gravity adds the character's own movement.yaccel, not a
// match-wide value.
func TestTick_GravityIsTheCharactersYAccel(t *testing.T) {
	got := tickOne(t, cns.PhysicsAir, "[Movement]\nyaccel = 0.6\n", match.Velocity{})
	if got.Velocity.Y != -float64(float32(0.6)) {
		t.Errorf("velocity y = %v, want -0.6 (yaccel), not TickConfig.Gravity", got.Velocity.Y)
	}
}

// char.go:10121-10137: physics S multiplies the x velocity by
// stand.friction after moving, zeroing it under 1; physics C by
// crouch.friction.
func TestTick_StandAndCrouchFriction(t *testing.T) {
	consts := "[Movement]\nstand.friction = 0.5\ncrouch.friction = 0.25\n"
	if got := tickOne(t, cns.PhysicsStanding, consts, match.Velocity{X: 4}); got.Velocity.X != 2 || got.Position.X != -6 {
		t.Errorf("stand: x %v vel %v, want -6 then 2", got.Position.X, got.Velocity.X)
	}
	if got := tickOne(t, cns.PhysicsStanding, consts, match.Velocity{X: 1.5}); got.Velocity.X != 0 {
		t.Errorf("stand under 1 after friction: vel %v, want 0", got.Velocity.X)
	}
	if got := tickOne(t, cns.PhysicsCrouching, consts, match.Velocity{X: 4}); got.Velocity.X != 1 {
		t.Errorf("crouch: vel %v, want 1", got.Velocity.X)
	}
	if got := tickOne(t, cns.PhysicsNone, consts, match.Velocity{X: 4}); got.Velocity.X != 4 {
		t.Errorf("physics N: vel %v, want 4 kept", got.Velocity.X)
	}
	if got := tickOne(t, cns.PhysicsStanding, "", match.Velocity{X: 4}); got.Velocity.X != 4*float64(float32(0.85)) {
		t.Errorf("default stand.friction: vel %v, want 3.4", got.Velocity.X)
	}
}
