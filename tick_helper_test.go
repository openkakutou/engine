package engine

import (
	"strings"
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/match"
)

func ctrlWith(typ string, triggers string, params map[string]string) cns.Controller {
	return cns.Controller{Type: typ, Triggers: []string{triggers}, Parameters: params}
}

// spawnerProgram: state 0 spawns helper 5 into state 1000 on its first tick,
// sets var(1) = 7 on its third; state 1000 moves forward 2 a tick and goes to
// 1001 once root,var(1) = 7; 1001 destroys the helper on its second tick.
// extra controllers go into state 0.
func spawnerProgram(helper map[string]string, extra ...cns.Controller) FighterProgram {
	h := map[string]string{"id": "5", "stateno": "1000", "pos": "30, 0"}
	for k, v := range helper {
		h[k] = v
	}
	zero := append([]cns.Controller{
		ctrlWith("Helper", "Time = 0", h),
		ctrlWith("VarSet", "Time = 2", map[string]string{"var(1)": "7"}),
	}, extra...)
	states := map[int]cns.StateDef{
		0: {Number: 0, Type: cns.StateTypeStanding, Ctrl: true, Controllers: zero},
		1000: {Number: 1000, Type: cns.StateTypeStanding, Anim: 100, Controllers: []cns.Controller{
			ctrlWith("VelSet", "1", map[string]string{"x": "2"}),
			ctrlWith("ChangeState", "root,var(1) = 7", map[string]string{"value": "1001"}),
		}},
		1001: {Number: 1001, Type: cns.StateTypeStanding, Anim: 100, Controllers: []cns.Controller{
			ctrlWith("DestroySelf", "Time = 1", nil),
		}},
	}
	return FighterProgram{States: states, Animations: []air.Animation{
		{Number: 0, Frames: []air.Frame{{Time: -1}}},
		{Number: 100, Frames: []air.Frame{{Time: -1, Clsn1: projBox}}},
	}}
}

// char.go:7214-7290 helperInit: at the spawner's position plus pos, its
// facing; the new helper runs in the same tick (actionPrepare, then
// charList.action), so it has already moved.
func TestTick_AHelperSpawnsAndRunsItsOwnStates(t *testing.T) {
	state, progs, rts := projMatch(t, spawnerProgram(nil), 900)
	_, all := runProj(t, state, progs, rts, 2)
	hs := all[0].Helpers[match.SideP1]
	if len(hs) != 1 {
		t.Fatalf("helpers after tick 1 = %d, want 1", len(hs))
	}
	h := hs[0].Context
	if !h.IsHelper || h.HelperID != 5 || h.StateNo != 1000 || h.Anim != 100 || h.Facing != match.FacingRight || h.Position.X != 32 {
		t.Errorf("helper = id %d state %d anim %d facing %v x %v, want 5, 1000, 100, right, 32", h.HelperID, h.StateNo, h.Anim, h.Facing, h.Position.X)
	}
	if x := all[1].Helpers[match.SideP1][0].Context.Position.X; x != 34 {
		t.Errorf("tick 2 x = %v, want 34", x)
	}
	if n := all[1].Fighters[match.SideP1].Context.Helpers; len(n) != 1 || n[0].HelperID != 5 {
		t.Errorf("the player's helpers = %+v, want helper 5 (NumHelper, helper(5))", n)
	}
}

func TestTick_AHelperReadsItsRootAndDestroysItself(t *testing.T) {
	state, progs, rts := projMatch(t, spawnerProgram(nil), 900)
	_, all := runProj(t, state, progs, rts, 6)
	if hs := all[2].Helpers[match.SideP1]; len(hs) != 1 || hs[0].Context.StateNo != 1001 {
		t.Fatalf("tick 3 helpers = %d, want one in state 1001 (root,var(1) = 7 since the player's tick 3)", len(hs))
	}
	if n := len(all[3].Helpers[match.SideP1]); n != 0 {
		t.Errorf("tick 4 helpers = %d, want 0 after DestroySelf at Time = 1", n)
	}
}

func TestTick_AHelperWithANegativeFacingFacesAway(t *testing.T) {
	state, progs, rts := projMatch(t, spawnerProgram(map[string]string{"facing": "-1"}), 900)
	_, all := runProj(t, state, progs, rts, 1)
	h := all[0].Helpers[match.SideP1][0].Context
	if h.Facing != match.FacingLeft || h.Position.X != 28 {
		t.Errorf("helper facing %v x %v, want left, 28 (30 ahead, then 2 back toward its facing)", h.Facing, h.Position.X)
	}
}

func TestTick_AHelpersHitDefHitsTheOpponent(t *testing.T) {
	prog := spawnerProgram(nil)
	s := prog.States[1000]
	s.MoveType = cns.MoveTypeAttack
	s.Controllers = append(s.Controllers, ctrlWith("HitDef", "1", map[string]string{"attr": "S, SA", "damage": "25"}))
	prog.States[1000] = s
	state, progs, rts := projMatch(t, prog, 40)
	final, _ := runProj(t, state, progs, rts, 1)
	if h := final.State.Fighters[match.SideP2].Health; h != 975 {
		t.Errorf("defender life = %d, want 975", h)
	}
	h := final.Helpers[match.SideP1][0].Context
	if h.MoveContactTime != 1 || final.Fighters[match.SideP1].Context.MoveContactTime != 0 {
		t.Errorf("contact: helper %d, player %d; want the helper's only", h.MoveContactTime, final.Fighters[match.SideP1].Context.MoveContactTime)
	}
}

func TestTick_AHelpersProjectileBelongsToThePlayer(t *testing.T) {
	prog := spawnerProgram(nil)
	s := prog.States[1000]
	s.Controllers = append([]cns.Controller{ctrlWith("Projectile", "Time = 0", map[string]string{"projanim": "100", "velocity": "1, 0"})}, s.Controllers...)
	prog.States[1000] = s
	state, progs, rts := projMatch(t, prog, 900)
	_, all := runProj(t, state, progs, rts, 1)
	if n := len(all[0].Projectiles[match.SideP1]); n != 1 {
		t.Errorf("player projectiles = %d, want 1 (thrown by its helper)", n)
	}
}

// char.go:7172-7178: past HelperMax (56, defaultConfig.ini) a Helper
// controller does nothing but warn.
func TestTick_HelpersAreCappedAtHelperMax(t *testing.T) {
	prog := spawnerProgram(map[string]string{"stateno": "1001"})
	zero := prog.States[0]
	zero.Controllers[0].Triggers = []string{"1"}
	prog.States[0] = zero
	one := prog.States[1001]
	one.Controllers = nil // never destroyed
	prog.States[1001] = one
	state, progs, rts := projMatch(t, prog, 900)
	_, all := runProj(t, state, progs, rts, 60)
	if n := len(all[59].Helpers[match.SideP1]); n != 56 {
		t.Errorf("helpers = %d, want 56", n)
	}
	warned := false
	for _, w := range all[56].Warnings[match.SideP1] {
		warned = warned || strings.Contains(w, "Reached limit of 56 helpers")
	}
	if !warned {
		t.Errorf("tick 57 warnings = %v, want the helper limit", all[56].Warnings[match.SideP1])
	}
}

// A helper's spawn into a state its character lacks warns and is skipped.
func TestTick_AHelperIntoAMissingStateIsSkippedWithAWarning(t *testing.T) {
	state, progs, rts := projMatch(t, spawnerProgram(map[string]string{"stateno": "4242"}), 900)
	_, all := runProj(t, state, progs, rts, 1)
	if n := len(all[0].Helpers[match.SideP1]); n != 0 {
		t.Errorf("helpers = %d, want 0", n)
	}
	if len(all[0].Warnings[match.SideP1]) == 0 {
		t.Error("want a warning naming the missing state")
	}
}
