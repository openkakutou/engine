package engine

import (
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

var projBox = []air.ClsnBox{{Left: -5, Top: -50, Right: 5, Bottom: 0}}

// casterProgram's state 0 throws, on its first tick, a projectile with
// animation 100 (a hit box), the given extra parameters.
func casterProgram(params map[string]string) FighterProgram {
	p := map[string]string{"projanim": "100", "velocity": "4, 0", "attr": "S, SP", "damage": "40", "offset": "20, 0"}
	for k, v := range params {
		p[k] = v
	}
	states := map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: []cns.Controller{
		{Type: "Projectile", Triggers: []string{"Time = 0"}, Parameters: p},
	}}}
	return FighterProgram{States: states, Animations: []air.Animation{
		{Number: 0, Frames: []air.Frame{{Time: -1}}},
		{Number: 100, Frames: []air.Frame{{Time: -1, Clsn1: projBox}}},
	}}
}

func projMatch(t *testing.T, caster FighterProgram, defenderX float64) (*match.MatchState, [2]FighterProgram, [2]FighterRuntime) {
	t.Helper()
	p1 := match.FighterState{Side: match.SideP1, Position: match.Position{X: 0}, Facing: match.FacingRight, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: defenderX}, Facing: match.FacingLeft, Health: 1000}
	progs := [2]FighterProgram{caster, hurtProgram()}
	r1, err := progs[0].NewRuntime(p1)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := progs[1].NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	state, err := match.NewMatchState(1, 0, p1, p2)
	if err != nil {
		t.Fatal(err)
	}
	return state, progs, [2]FighterRuntime{r1, r2}
}

func runProj(t *testing.T, state *match.MatchState, progs [2]FighterProgram, rts [2]FighterRuntime, ticks int) (TickResult, []TickResult) {
	t.Helper()
	var all []TickResult
	var res TickResult
	s := *state
	for i := 1; i <= ticks; i++ {
		var err error
		res, err = Tick(s, progs, rts, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Tick: i, ComboWindow: 60})
		if err != nil {
			t.Fatal(err)
		}
		s, rts = res.State, res.Fighters
		all = append(all, res)
	}
	return res, all
}

// char.go Projectile: spawned at the caster's position plus offset, facing
// its way, moving by velocity each tick.
func TestTick_AProjectileSpawnsAndMoves(t *testing.T) {
	state, progs, rts := projMatch(t, casterProgram(nil), 900)
	_, all := runProj(t, state, progs, rts, 3)
	views := all[2].Projectiles[match.SideP1]
	if len(views) != 1 {
		t.Fatalf("projectiles = %+v, want one", all[2].Projectiles)
	}
	v := views[0]
	if v.Anim != 100 || v.Facing != match.FacingRight || v.Position.X != 20+4*3 {
		t.Errorf("projectile = %+v, want anim 100 facing right at 32 (20, then 4 a tick from the spawn tick on, system.go charUpdate)", v)
	}
}

func TestTick_AProjectileHitsTheOpponentAndIsRemoved(t *testing.T) {
	state, progs, rts := projMatch(t, casterProgram(nil), 60)
	final, _ := runProj(t, state, progs, rts, 20)
	if h := final.State.Fighters[match.SideP2].Health; h != 960 {
		t.Errorf("defender life = %d, want 960 (damage 40)", h)
	}
	if n := len(final.Projectiles[match.SideP1]); n != 0 {
		t.Errorf("projectiles after the hit = %d, want 0 (projhitanim -1)", n)
	}
}

func TestTick_AProjectileIsRemovedByItsRemoveTime(t *testing.T) {
	state, progs, rts := projMatch(t, casterProgram(map[string]string{"projremovetime": "3"}), 900)
	_, all := runProj(t, state, progs, rts, 6)
	if n := len(all[1].Projectiles[match.SideP1]); n != 1 {
		t.Errorf("tick 2: %d projectiles, want 1", n)
	}
	if n := len(all[5].Projectiles[match.SideP1]); n != 0 {
		t.Errorf("tick 6: %d projectiles, want 0 after projremovetime 3", n)
	}
}

func TestTick_AProjectileLeavingTheScreenIsRemoved(t *testing.T) {
	state, progs, rts := projMatch(t, casterProgram(map[string]string{"velocity": "100, 0"}), 900)
	final, _ := runProj(t, state, progs, rts, 30)
	if n := len(final.Projectiles[match.SideP1]); n != 0 {
		t.Errorf("%d projectiles after flying off for 30 ticks, want 0", n)
	}
}

// ProjHitTime(id) (char.go projHitTime) is -1 before the projectile with
// that id hits and counts from 0 the tick after; NumProjID counts them.
func TestTick_ProjHitTimeAndNumProjID(t *testing.T) {
	prog := casterProgram(map[string]string{"projid": "1000"})
	zero := prog.States[0]
	zero.Controllers = append(zero.Controllers,
		ctrlWith("VarSet", "1", map[string]string{"var(0)": "ProjHitTime(1000)"}),
		ctrlWith("VarSet", "1", map[string]string{"var(1)": "NumProjID(1000)"}))
	prog.States[0] = zero
	state, progs, rts := projMatch(t, prog, 60)
	_, all := runProj(t, state, progs, rts, 20)
	if v := all[1].Fighters[match.SideP1].Context.Vars[1]; v != 1 {
		t.Errorf("tick 2 NumProjID(1000) = %d, want 1", v)
	}
	hitTick := -1
	for i, r := range all {
		if hitTick < 0 && r.State.Fighters[match.SideP2].Health < 1000 {
			hitTick = i
		}
	}
	if hitTick < 0 {
		t.Fatal("the projectile never hit")
	}
	if v := all[hitTick].Fighters[match.SideP1].Context.Vars[0]; v != -1 {
		t.Errorf("on the hit tick ProjHitTime = %d, want -1 (read before the hit)", v)
	}
	if v := all[hitTick+1].Fighters[match.SideP1].Context.Vars[0]; v != 0 {
		t.Errorf("the tick after ProjHitTime = %d, want 0", v)
	}
	if v := all[hitTick+3].Fighters[match.SideP1].Context.Vars[0]; v != 2 {
		t.Errorf("three ticks after ProjHitTime = %d, want 2", v)
	}
}

// char.go:12194-12198, 14525-14535: a projectile's hit never turns its
// owner (p1facing is for the player's own HitDef) nor adds to its hit count.
func TestTick_AProjectilesHitLeavesItsOwnersFacingAndHitCount(t *testing.T) {
	state, progs, rts := projMatch(t, casterProgram(map[string]string{"p1facing": "-1"}), 60)
	final, _ := runProj(t, state, progs, rts, 20)
	if h := final.State.Fighters[match.SideP2].Health; h != 960 {
		t.Fatalf("defender life = %d, want 960 (the projectile hit)", h)
	}
	p := final.Fighters[match.SideP1].Context
	if p.Facing != match.FacingRight || p.HitCount != 0 {
		t.Errorf("owner facing %v hit count %d, want right, 0", p.Facing, p.HitCount)
	}
}
