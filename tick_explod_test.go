package engine

import (
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/match"
)

// explodProgram's state 0 makes an explod on its first tick with the given
// parameters (anim 100, a 4 + 4 tick animation, unless given).
func explodProgram(params map[string]string, extra ...cns.Controller) FighterProgram {
	p := map[string]string{"anim": "100"}
	for k, v := range params {
		p[k] = v
	}
	controllers := append([]cns.Controller{ctrlWith("Explod", "Time = 0", p)}, extra...)
	states := map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: controllers}}
	return FighterProgram{States: states, Animations: []air.Animation{
		{Number: 0, Frames: []air.Frame{{Time: -1}}},
		{Number: 100, Frames: []air.Frame{{Time: 4}, {Time: 4}}},
		{Number: 101, Frames: []air.Frame{{Time: -1}}},
	}}
}

// char.go:1961-2000 setPos (postype p1): at the fighter plus pos, x toward
// its facing, y up for a negative pos y; facing the fighter's way.
func TestTick_AnExplodAppearsAtTheFighterPlusPos(t *testing.T) {
	state, progs, rts := projMatch(t, explodProgram(map[string]string{"pos": "20, -30", "id": "7"}), 900)
	_, all := runProj(t, state, progs, rts, 1)
	es := all[0].Explods[match.SideP1]
	if len(es) != 1 {
		t.Fatalf("explods = %d, want 1", len(es))
	}
	e := es[0]
	if e.ID != 7 || e.Anim != 100 || e.Common || e.Position.X != 20 || e.Position.Y != 30 || e.Facing != match.FacingRight {
		t.Errorf("explod = %+v, want id 7, anim 100 own, at 20,30 facing right", e)
	}
}

// char.go:2146-2154: removetime -2 (the default) removes it once its
// animation has played; a set removetime once its time reaches it.
func TestTick_AnExplodGoesWithItsAnimationOrItsRemoveTime(t *testing.T) {
	state, progs, rts := projMatch(t, explodProgram(nil), 900)
	_, all := runProj(t, state, progs, rts, 10)
	if n := len(all[6].Explods[match.SideP1]); n != 1 {
		t.Errorf("tick 7: %d explods, want 1 (8-tick animation)", n)
	}
	if n := len(all[9].Explods[match.SideP1]); n != 0 {
		t.Errorf("tick 10: %d explods, want 0 after its animation", n)
	}
	state, progs, rts = projMatch(t, explodProgram(map[string]string{"anim": "101", "removetime": "3"}), 900)
	_, all = runProj(t, state, progs, rts, 6)
	if n := len(all[2].Explods[match.SideP1]); n != 1 {
		t.Errorf("removetime 3, tick 3: %d explods, want 1", n)
	}
	if n := len(all[4].Explods[match.SideP1]); n != 0 {
		t.Errorf("removetime 3, tick 5: %d explods, want 0", n)
	}
	state, progs, rts = projMatch(t, explodProgram(map[string]string{"anim": "101", "removetime": "-1"}), 900)
	_, all = runProj(t, state, progs, rts, 40)
	if n := len(all[39].Explods[match.SideP1]); n != 1 {
		t.Errorf("removetime -1: %d explods after 40 ticks, want 1", n)
	}
}

// char.go:2287-2300: once unbound it moves by its velocity toward its
// facing, its velocity growing by its acceleration.
func TestTick_AnExplodMovesByItsVelocity(t *testing.T) {
	state, progs, rts := projMatch(t, explodProgram(map[string]string{"anim": "101", "removetime": "-1", "vel": "2, -1", "accel": "1, 0", "bindtime": "0"}), 900)
	_, all := runProj(t, state, progs, rts, 3)
	e := all[2].Explods[match.SideP1][0]
	if e.Position.X != 2+3 || e.Position.Y != 2 {
		t.Errorf("explod at %v,%v, want 5,2 (x: 2 then 3; y up 1 a tick)", e.Position.X, e.Position.Y)
	}
}

// RemoveExplod (char.go:7468) removes the fighter's explods with that id.
func TestTick_RemoveExplodRemovesThatID(t *testing.T) {
	prog := explodProgram(map[string]string{"anim": "101", "removetime": "-1", "id": "3"},
		ctrlWith("Explod", "Time = 0", map[string]string{"anim": "101", "removetime": "-1", "id": "4"}),
		ctrlWith("RemoveExplod", "Time = 2", map[string]string{"id": "3"}),
		ctrlWith("VarSet", "1", map[string]string{"var(0)": "NumExplod(4) * 10 + NumExplod"}))
	state, progs, rts := projMatch(t, prog, 900)
	_, all := runProj(t, state, progs, rts, 4)
	es := all[3].Explods[match.SideP1]
	if len(es) != 1 || es[0].ID != 4 {
		t.Errorf("explods = %+v, want only id 4", es)
	}
	if v := all[3].Fighters[match.SideP1].Context.Vars[0]; v != 11 {
		t.Errorf("NumExplod(4)*10 + NumExplod = %d, want 11", v)
	}
}

// An F anim is the common fightfx's (bytecode.go explod_anim): the engine
// keeps it as such for the game to draw.
func TestTick_AnFExplodIsCommon(t *testing.T) {
	state, progs, rts := projMatch(t, explodProgram(map[string]string{"anim": "F60", "removetime": "5"}), 900)
	_, all := runProj(t, state, progs, rts, 1)
	es := all[0].Explods[match.SideP1]
	if len(es) != 1 || !es[0].Common || es[0].Anim != 60 {
		t.Errorf("explods = %+v, want common anim 60", es)
	}
}

// char.go:7309-7315: past ExplodMax (512, defaultConfig.ini) none is made.
func TestTick_ExplodsAreCappedAtExplodMax(t *testing.T) {
	prog := explodProgram(map[string]string{"anim": "101", "removetime": "-1"})
	zero := prog.States[0]
	zero.Controllers[0].Triggers = []string{"1"}
	for i := 0; i < 9; i++ {
		zero.Controllers = append(zero.Controllers, zero.Controllers[0])
	}
	prog.States[0] = zero
	state, progs, rts := projMatch(t, prog, 900)
	_, all := runProj(t, state, progs, rts, 60)
	if n := len(all[59].Explods[match.SideP1]); n != 512 {
		t.Errorf("explods = %d, want 512", n)
	}
}

// ModifyExplod (bytecode.go:7158-7560) changes the live explods of that id:
// here a new velocity and a remove time.
func TestTick_ModifyExplodChangesThatIDsExplods(t *testing.T) {
	prog := explodProgram(map[string]string{"anim": "101", "removetime": "-1", "id": "3", "bindtime": "0"},
		ctrlWith("Explod", "Time = 0", map[string]string{"anim": "101", "removetime": "-1", "id": "4", "bindtime": "0"}),
		ctrlWith("ModifyExplod", "Time = 1", map[string]string{"id": "3", "postype": "p1", "vel": "5, 0", "removetime": "6"}))
	state, progs, rts := projMatch(t, prog, 900)
	_, all := runProj(t, state, progs, rts, 8)
	es := all[3].Explods[match.SideP1]
	if len(es) != 2 {
		t.Fatalf("explods = %+v, want both", es)
	}
	moved, still := es[0], es[1]
	if moved.ID != 3 {
		moved, still = still, moved
	}
	if moved.Vel[0] != 5 || still.Vel[0] != 0 {
		t.Errorf("velocities: id 3 %v, id 4 %v, want 5 and 0", moved.Vel, still.Vel)
	}
	if es := all[7].Explods[match.SideP1]; len(es) != 1 || es[0].ID != 4 {
		t.Errorf("after its new remove time: %+v, want only id 4", es)
	}
}

// P2's TargetBind moves P1 before P2's explods are placed: a postype p2
// explod made by P2 the same tick appears at P1's bound position, as P1's
// own explods see P2 after its tick.
func TestTick_P2sPostypeP2ExplodSeesP1WhereItsTargetBindPutIt(t *testing.T) {
	binder := explodProgram(map[string]string{"postype": "p2", "pos": "0, 0"},
		ctrlWith("TargetBind", "1", map[string]string{"pos": "30, 0", "time": "1"}))
	state, progs, rts := projMatch(t, hurtProgram(), 900)
	progs[match.SideP2] = binder
	p2, err := binder.NewRuntime(state.Fighters[match.SideP2])
	if err != nil {
		t.Fatal(err)
	}
	p2.Context.HasTarget = true
	rts[match.SideP2] = p2
	rts[match.SideP1].Context.MoveType = 'H'
	final, _ := runProj(t, state, progs, rts, 1)
	if x := final.State.Fighters[match.SideP1].Position.X; x != 870 {
		t.Fatalf("P1 at x %v, want 870 (bound 30 in front of P2)", x)
	}
	es := final.Explods[match.SideP2]
	if len(es) != 1 || es[0].Position.X != 870 {
		t.Errorf("P2's explods = %+v, want one at x 870 (P1 after the bind)", es)
	}
}
