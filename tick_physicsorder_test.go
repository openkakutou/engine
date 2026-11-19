package engine

import (
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/match"
)

// Item 044: the order inside a tick follows Ikemen GO (commit 54d4148a).
// actionRun runs a fighter's states, then posUpdate -- position from velocity,
// then gravity in physics A (char.go:12583, 10102-10139) -- then the landing
// check (char.go:12584-12590); charList.action does it fighter by fighter
// (char.go:14080-14084), and collision detection comes after
// (system.go:2734). See decision 029.

// Fighter 2 reads fighter 1 after fighter 1 has moved: P1 at -10 with VelSet
// x = 3 is at -7 when P2 runs, so the left-facing P2 at 30 reads
// P2Dist X = -1 * (-7 - 30) = 37.
func TestTick_SecondFighterSeesTheFirstOnesPositionAfterItsMovement(t *testing.T) {
	p1States := stateWith(ctrl("VelSet", "x", "3"))
	p2States := map[int]cns.StateDef{0: standing(0, recordVar("0", "P2Dist X"))}
	p1 := match.FighterState{Facing: match.FacingRight, Position: match.Position{X: -10}, Health: 1000}
	p2 := match.FighterState{Facing: match.FacingLeft, Position: match.Position{X: 30}, Health: 1000}
	res := tickPair(t, [2]FighterProgram{{States: p1States}, {States: p2States}}, [2]match.FighterState{p1, p2}, 0)
	if got := res.Fighters[match.SideP2].Context.Vars[0]; got != 37 {
		t.Errorf("P2 read P2Dist X = %d, want 37 (P1 already moved to -7)", got)
	}
}

// Hit detection sees the attacker where its velocity has just taken it: the
// hit scenario's attacker (X 0, velocity 3) reaches a defender up to 15 away
// from where it stands, so a defender at 17 is hit only from X 3.
func TestTick_HitDetection_SeesTheAttackerAfterThisTicksMovement(t *testing.T) {
	s := newHitScenario(t, nil)
	s.state.Fighters[match.SideP2].Position.X = 17
	s.runtimes[match.SideP2].Context.FighterState.Position.X = 17
	s.step()
	if got := s.ctx(match.SideP1).MoveContactTime; got != 1 {
		t.Errorf("MoveContactTime = %d, want 1 (the hit lands from X 3)", got)
	}
}

// landingStates is a physics A state 0, state 52 (Ikemen GO's landing) and a
// physics A state 105.
func landingStates() map[int]cns.StateDef {
	states := physicsState(cns.PhysicsAir)
	states[52] = cns.StateDef{Number: 52, Type: cns.StateTypeStanding, Physics: cns.PhysicsStanding, Anim: 47}
	states[105] = cns.StateDef{Number: 105, Type: cns.StateTypeAir, Physics: cns.PhysicsAir}
	return states
}

// Ikemen GO char.go:12584-12590: after posUpdate, a physics A fighter falling
// (vel y down) at or below the ground, outside state 105, changes to state 52.
// From Y 1 falling 2 per tick with yaccel 0.5: Y -1, then the landing.
func TestTick_PhysicsA_FallingThroughTheGround_EntersState52OnTheGround(t *testing.T) {
	prog := FighterProgram{States: landingStates()}
	res := tickWithP1Program(t, prog, match.FighterState{Position: match.Position{X: 5, Y: 1}, Velocity: match.Velocity{X: 2, Y: -2}, Health: 1000}, 0.5)
	got := res.State.Fighters[match.SideP1]
	ctx := res.Fighters[match.SideP1].Context
	if got.StateNo != 52 || ctx.StateNo != 52 {
		t.Fatalf("StateNo = %d (context %d), want 52", got.StateNo, ctx.StateNo)
	}
	if got.Position.Y != 0 || got.Velocity.Y != 0 {
		t.Errorf("Y, Vel Y = %v, %v, want 0, 0 (landed)", got.Position.Y, got.Velocity.Y)
	}
	if got.Position.X != 7 || got.Velocity.X != 2 {
		t.Errorf("X, Vel X = %v, %v, want 7, 2 (the landing keeps the horizontal velocity)", got.Position.X, got.Velocity.X)
	}
	if ctx.Physics != 'S' || ctx.Anim != 47 || ctx.Time != 0 || ctx.PrevStateNo != 0 {
		t.Errorf("Physics, Anim, Time, PrevStateNo = %c, %d, %d, %d, want S, 47, 0, 0 (state 52 entered)", ctx.Physics, ctx.Anim, ctx.Time, ctx.PrevStateNo)
	}
}

func TestTick_PhysicsA_StillAboveTheGround_DoesNotLand(t *testing.T) {
	prog := FighterProgram{States: landingStates()}
	got := p1After(t, prog, match.FighterState{Position: match.Position{Y: 3}, Velocity: match.Velocity{Y: -2}, Health: 1000}, 0.5)
	if got.StateNo != 0 || got.Position.Y != 1 || got.Velocity.Y != -2.5 {
		t.Errorf("StateNo, Y, Vel Y = %d, %v, %v, want 0, 1, -2.5", got.StateNo, got.Position.Y, got.Velocity.Y)
	}
}

// Below the ground but rising is not a landing: vel y must point down.
func TestTick_PhysicsA_RisingFromTheGround_DoesNotLand(t *testing.T) {
	prog := FighterProgram{States: landingStates()}
	got := p1After(t, prog, match.FighterState{Position: match.Position{Y: -2}, Velocity: match.Velocity{Y: 1}, Health: 1000}, 0)
	if got.StateNo != 0 || got.Position.Y != -1 {
		t.Errorf("StateNo, Y = %d, %v, want 0, -1 (below the ground but rising)", got.StateNo, got.Position.Y)
	}
}

func TestTick_PhysicsA_InState105_DoesNotLand(t *testing.T) {
	states := landingStates()
	states[0] = changeToState(states[0], "105")
	prog := FighterProgram{States: states}
	got := p1After(t, prog, match.FighterState{Position: match.Position{Y: 1}, Velocity: match.Velocity{Y: -2}, Health: 1000}, 0.5)
	if got.StateNo != 105 || got.Position.Y != -1 {
		t.Errorf("StateNo, Y = %d, %v, want 105, -1 (state 105 is excluded, char.go:12587)", got.StateNo, got.Position.Y)
	}
}

func TestTick_PhysicsS_BelowTheGround_DoesNotLand(t *testing.T) {
	states := landingStates()
	def := states[0]
	def.Physics = cns.PhysicsStanding
	states[0] = def
	got := p1After(t, FighterProgram{States: states}, match.FighterState{Position: match.Position{Y: 1}, Velocity: match.Velocity{Y: -2}, Health: 1000}, 0.5)
	if got.StateNo != 0 {
		t.Errorf("StateNo = %d, want 0 (only physics A lands)", got.StateNo)
	}
}

// changeToState appends an unconditional ChangeState to def.
func changeToState(def cns.StateDef, value string) cns.StateDef {
	def.Controllers = append(def.Controllers, changeTo(value, "1"))
	return def
}
