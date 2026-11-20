package engine

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/combat"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// parseStatesFiles parses the given testdata .cns files in order into one
// state map; a later file's StateDef replaces an earlier one's of the same
// number.
func parseStatesFiles(t *testing.T, names ...string) map[int]cns.StateDef {
	t.Helper()
	out := make(map[int]cns.StateDef)
	for _, name := range names {
		f, err := os.Open(filepath.Join("testdata", name))
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		states, err := cns.Parse(f)
		f.Close()
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, s := range states {
			out[s.Number] = s
		}
	}
	return out
}

func loadFighterAnimations(t *testing.T) []air.Animation {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "fighter.air"))
	if err != nil {
		t.Fatalf("open fighter.air: %v", err)
	}
	defer f.Close()
	anims, err := air.Parse(f)
	if err != nil {
		t.Fatalf("parse fighter.air: %v", err)
	}
	return anims
}

func combatHit(defender match.Side) combat.HitResult {
	return combat.HitResult{Hit: true, Defender: defender}
}

// countVarSet is a controller incrementing var(slot) on every tick it runs:
// the observable proof that a fighter's controllers ran (or did not).
func countVarSet(slot string) cns.Controller {
	return cns.Controller{
		Type:       "VarSet",
		Parameters: map[string]string{"v": slot, "value": "var(" + slot + ") + 1"},
	}
}

func withCounter(def cns.StateDef, slot string) cns.StateDef {
	def.Controllers = append([]cns.Controller{countVarSet(slot)}, def.Controllers...)
	return def
}

// hitScenario is P1 (in the real kfm state 200, its HitDef firing at Time 0,
// moving right at 3/tick) overlapping P2 (idle in state 0), so the hit lands
// on the very first Tick.
type hitScenario struct {
	t        *testing.T
	state    match.MatchState
	programs [2]FighterProgram
	runtimes [2]FighterRuntime
	tick     int
	last     TickResult
	in       [2]input.TickInput // fed to every step
}

func newHitScenario(t *testing.T, tweak func(attacker, defender map[int]cns.StateDef)) *hitScenario {
	t.Helper()
	attacker := parseStatesFiles(t, "fighter.cns", "kfm_hitdef.cns")
	defender := parseStatesFiles(t, "fighter.cns", "common1_gethit.cns")
	attacker[200] = withCounter(attacker[200], "0")
	defender[0] = withCounter(defender[0], "1")
	defender[5000] = withCounter(defender[5000], "1")
	if tweak != nil {
		tweak(attacker, defender)
	}
	anims := loadFighterAnimations(t)

	p1 := match.FighterState{Side: match.SideP1, Facing: match.FacingRight, Health: 1000, StateNo: 200, Position: match.Position{X: 0}, Velocity: match.Velocity{X: 3}}
	p2 := match.FighterState{Side: match.SideP2, Facing: match.FacingLeft, Health: 1000, Position: match.Position{X: 5}}
	ms, err := match.NewMatchState(1, 1000, p1, p2)
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}
	r1, err := NewFighterRuntime(p1, attacker)
	if err != nil {
		t.Fatalf("NewFighterRuntime(p1): %v", err)
	}
	r2, err := NewFighterRuntime(p2, defender)
	if err != nil {
		t.Fatalf("NewFighterRuntime(p2): %v", err)
	}
	return &hitScenario{
		t:        t,
		state:    *ms,
		programs: [2]FighterProgram{match.SideP1: {States: attacker, Animations: anims}, match.SideP2: {States: defender, Animations: anims}},
		runtimes: [2]FighterRuntime{match.SideP1: r1, match.SideP2: r2},
	}
}

func (s *hitScenario) step() TickResult {
	s.t.Helper()
	s.tick++
	res, err := Tick(s.state, s.programs, s.runtimes, s.in, TickConfig{Bounds: testBounds(), Gravity: 0, Tick: s.tick, ComboWindow: 60})
	if err != nil {
		s.t.Fatalf("Tick %d: %v", s.tick, err)
	}
	s.state, s.runtimes, s.last = res.State, res.Fighters, res
	return res
}

func (s *hitScenario) ctx(side match.Side) evaluator.Context { return s.runtimes[side].Context }

func withPausetime(v string) func(a, d map[int]cns.StateDef) {
	return func(a, d map[int]cns.StateDef) {
		setHitDefParam(a, "pausetime", v)
	}
}

func setHitDefParam(states map[int]cns.StateDef, key, value string) {
	def := states[200]
	ctrls := make([]cns.Controller, len(def.Controllers))
	copy(ctrls, def.Controllers)
	for i := range ctrls {
		if ctrls[i].Type == "HitDef" {
			params := make(map[string]string, len(ctrls[i].Parameters))
			for k, v := range ctrls[i].Parameters {
				params[k] = v
			}
			if value == "" {
				delete(params, key)
			} else {
				params[key] = value
			}
			ctrls[i].Parameters = params
		}
	}
	def.Controllers = ctrls
	states[200] = def
}

func TestTick_LandedHit_FreezesBothFightersForTheKfmPauseTime_HoldingTimeAnimTimePositionAndControllers(t *testing.T) {
	s := newHitScenario(t, nil)

	s.step() // tick 1: the hit lands (pausetime = 8, 8)
	attackerX := s.state.Fighter(match.SideP1).Position.X
	if attackerX != 3 {
		t.Fatalf("landing tick: P1 X = %v, want 3 (physics still runs on the landing tick)", attackerX)
	}
	if got := s.ctx(match.SideP1).Vars[0]; got != 1 {
		t.Fatalf("landing tick: attacker var(0) = %d, want 1", got)
	}
	attackerTime, attackerAnimTime := s.ctx(match.SideP1).Time, s.ctx(match.SideP1).AnimElapsed
	defenderCounter := s.ctx(match.SideP2).Vars[1] // 1: its state-0 counter ran before the hit landed

	// Ticks 2..9 are the 8 frozen ticks for both sides.
	for tick := 2; tick <= 9; tick++ {
		s.step()
		a, d := s.ctx(match.SideP1), s.ctx(match.SideP2)
		if a.Vars[0] != 1 || d.Vars[1] != defenderCounter {
			t.Fatalf("tick %d: attacker var(0)=%d defender var(1)=%d, want 1 and %d (no controller may run while frozen)", tick, a.Vars[0], d.Vars[1], defenderCounter)
		}
		if a.Time != attackerTime || a.AnimElapsed != attackerAnimTime {
			t.Fatalf("tick %d: attacker Time/AnimTime = %d/%d, want held at %d/%d", tick, a.Time, a.AnimElapsed, attackerTime, attackerAnimTime)
		}
		if d.Time != 0 || d.AnimElapsed != 0 || d.StateNo != 5000 {
			t.Fatalf("tick %d: defender StateNo/Time/AnimTime = %d/%d/%d, want 5000/0/0", tick, d.StateNo, d.Time, d.AnimElapsed)
		}
		if x := s.state.Fighter(match.SideP1).Position.X; x != attackerX {
			t.Fatalf("tick %d: attacker X = %v, want held at %v (physics skipped)", tick, x, attackerX)
		}
	}

	// Tick 10 is the first unfrozen tick of both sides.
	s.step()
	if got := s.ctx(match.SideP1).Vars[0]; got != 2 {
		t.Errorf("tick 10: attacker var(0) = %d, want 2 (controllers run again)", got)
	}
	// The velocity 3 left the stand friction once (0.85, char.go:10121-10127)
	// before the freeze.
	if want := attackerX + 3*float64(float32(0.85)); s.state.Fighter(match.SideP1).Position.X != want {
		t.Errorf("tick 10: attacker X = %v, want %v (physics runs again)", s.state.Fighter(match.SideP1).Position.X, want)
	}
	if got := s.ctx(match.SideP2).Vars[1]; got != defenderCounter+1 {
		t.Errorf("tick 10: defender var(1) = %d, want %d (5000's counter ran once)", got, defenderCounter+1)
	}
	if got := s.ctx(match.SideP2).StateNo; got != 5001 {
		t.Errorf("tick 10: defender StateNo = %d, want 5001 (HitShakeOver on the first tick after the shake)", got)
	}
}

func TestTick_LandedHit_FreezesEachSideForItsOwnPauseTime(t *testing.T) {
	s := newHitScenario(t, withPausetime("3, 6"))

	s.step() // tick 1: hit lands
	for tick := 2; tick <= 4; tick++ {
		s.step()
		if got := s.ctx(match.SideP1).Vars[0]; got != 1 {
			t.Fatalf("tick %d: attacker var(0) = %d, want 1 (frozen 3 ticks)", tick, got)
		}
	}
	s.step() // tick 5
	if got := s.ctx(match.SideP1).Vars[0]; got != 2 {
		t.Errorf("tick 5: attacker var(0) = %d, want 2 (attacker's pause is 3 ticks, not the defender's 6)", got)
	}
	if got := s.ctx(match.SideP2).StateNo; got != 5000 {
		t.Errorf("tick 5: defender StateNo = %d, want still 5000 (shaking 6 ticks)", got)
	}
	s.step() // tick 6
	s.step() // tick 7: last frozen tick for the defender
	if got := s.ctx(match.SideP2).StateNo; got != 5000 {
		t.Errorf("tick 7: defender StateNo = %d, want still 5000", got)
	}
	if got := s.ctx(match.SideP2).Vars[1]; got != 1 {
		t.Errorf("tick 7: defender var(1) = %d, want 1 (only the pre-hit tick ran; none during the shake)", got)
	}
	s.step() // tick 8
	if got := s.ctx(match.SideP2).StateNo; got != 5001 {
		t.Errorf("tick 8: defender StateNo = %d, want 5001", got)
	}
}

func TestTick_LandedHit_EntersTheGetHitStateMatchingTheDefendersStanceOnTheLandingTick(t *testing.T) {
	cases := []struct {
		name        string
		stateType   cns.StateType
		wantState   int
		wantHitTime int
	}{
		{"standing defender goes to 5000 with ground.hittime", cns.StateTypeStanding, 5000, 11},
		{"crouching defender goes to 5010 with ground.hittime", cns.StateTypeCrouching, 5010, 11},
		{"airborne defender goes to 5020 with air.hittime", cns.StateTypeAir, 5020, 15},
		{"lowercase type = s is standing", cns.StateType("s"), 5000, 11},
		{"lowercase type = c is crouching", cns.StateType("c"), 5010, 11},
		{"lowercase type = a is airborne, with air.hittime", cns.StateType("a"), 5020, 15},
		{"omitted type line defaults to standing (MUGEN: omitted = S)", cns.StateType(""), 5000, 11},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newHitScenario(t, func(a, d map[int]cns.StateDef) {
				def := d[0]
				def.Type = c.stateType
				def.Ctrl = true
				d[0] = def
			})
			s.step()
			d := s.ctx(match.SideP2)
			if d.StateNo != c.wantState {
				t.Errorf("defender StateNo = %d, want %d", d.StateNo, c.wantState)
			}
			if got := s.state.Fighter(match.SideP2).StateNo; got != c.wantState {
				t.Errorf("MatchState defender StateNo = %d, want %d", got, c.wantState)
			}
			if d.Time != 0 || d.AnimElapsed != 0 {
				t.Errorf("defender Time/AnimTime = %d/%d, want 0/0", d.Time, d.AnimElapsed)
			}
			if d.Anim != c.wantState {
				t.Errorf("defender Anim = %d, want %d (StateDef anim)", d.Anim, c.wantState)
			}
			if d.Ctrl {
				t.Errorf("defender Ctrl = true, want false (get-hit StateDef has no ctrl)")
			}
			if d.GetHit.HitTime != c.wantHitTime {
				t.Errorf("GetHit.HitTime = %d, want %d", d.GetHit.HitTime, c.wantHitTime)
			}
		})
	}
}

func TestTick_LandedHit_RecordsTheKfmGetHitVars(t *testing.T) {
	s := newHitScenario(t, nil)
	s.step()
	want := evaluator.GetHit{Active: true, HitShakeTime: 8, HitTime: 11, SlideTime: 5, AnimType: 0, GroundType: 1, AirType: 1,
		Yaccel: 0.35, FallYvel: -4.5, FallRecover: true, FallRecoverTime: 4, CtrlTime: 11}
	if got := s.ctx(match.SideP2).GetHit; got != want {
		t.Errorf("defender GetHit = %+v, want %+v", got, want)
	}
}

func TestTick_GetHitState_ReturnsToStandingOnceHitOver_AfterGroundHitTimePlusOneUnfrozenTicks(t *testing.T) {
	s := newHitScenario(t, nil)
	s.step() // tick 1: lands
	for tick := 2; tick <= 9; tick++ {
		s.step() // frozen
	}
	// ground.hittime = 11: hittime counts 11,10,...,0 on unfrozen ticks
	// 10..21 (it reads 0 while still "not over"), and is -1 (HitOver) at the
	// start of tick 22.
	for tick := 10; tick <= 21; tick++ {
		s.step()
		if got := s.ctx(match.SideP2).StateNo; got != 5001 {
			t.Fatalf("tick %d: defender StateNo = %d, want 5001", tick, got)
		}
	}
	s.step() // tick 22
	d := s.ctx(match.SideP2)
	if d.StateNo != 0 {
		t.Errorf("tick 22: defender StateNo = %d, want 0 (HitOver)", d.StateNo)
	}
	if !d.Ctrl {
		t.Errorf("tick 22: defender Ctrl = false, want true (state 0 declares ctrl = 1)")
	}
	if d.GetHit.HitTime != -1 {
		t.Errorf("tick 22: GetHit.HitTime = %d, want -1 (the countdown stops at -1)", d.GetHit.HitTime)
	}
	s.step()
	if got := s.ctx(match.SideP2).GetHit.HitTime; got != -1 {
		t.Errorf("tick 23: GetHit.HitTime = %d, want it to stay -1", got)
	}
}

func TestTick_LandedHit_WithNoHitDefParameters_UsesTheDocumentedDefaults(t *testing.T) {
	strip := func(a, d map[int]cns.StateDef) {
		for _, k := range []string{"pausetime", "ground.hittime", "ground.slidetime", "air.hittime", "animtype", "ground.type", "air.type"} {
			setHitDefParam(a, k, "")
		}
	}
	s := newHitScenario(t, strip)
	s.step()
	want := evaluator.GetHit{Active: true, HitShakeTime: 0, HitTime: 0, SlideTime: 0, AnimType: 0, GroundType: 1, AirType: 1,
		Yaccel: 0.35, FallYvel: -4.5, FallRecover: true, FallRecoverTime: 4}
	if got := s.ctx(match.SideP2).GetHit; got != want {
		t.Errorf("defender GetHit = %+v, want %+v", got, want)
	}
	s.step()
	if got := s.ctx(match.SideP1).Vars[0]; got != 2 {
		t.Errorf("tick 2: attacker var(0) = %d, want 2 (pausetime default 0 = no freeze)", got)
	}
	if got := s.ctx(match.SideP2).StateNo; got != 5001 {
		t.Errorf("tick 2: defender StateNo = %d, want 5001 (no shake)", got)
	}

	air := newHitScenario(t, func(a, d map[int]cns.StateDef) {
		strip(a, d)
		def := d[0]
		def.Type = cns.StateTypeAir
		d[0] = def
	})
	air.step()
	if got := air.ctx(match.SideP2).GetHit.HitTime; got != 20 {
		t.Errorf("airborne defender default hittime = %d, want 20 (air.hittime default)", got)
	}
}

func TestTick_LandedHit_ExpressionHitDefValueFallsBackToItsDefault(t *testing.T) {
	s := newHitScenario(t, func(a, d map[int]cns.StateDef) {
		setHitDefParam(a, "pausetime", "var(0), 8")
		setHitDefParam(a, "ground.hittime", "11 + 1")
	})
	s.step()
	got := s.ctx(match.SideP2).GetHit
	if got.HitShakeTime != 8 || got.HitTime != 0 {
		t.Errorf("GetHit = %+v, want HitShakeTime 8 (literal) and HitTime 0 (expression -> default)", got)
	}
	s.step()
	if v := s.ctx(match.SideP1).Vars[0]; v != 2 {
		t.Errorf("attacker var(0) = %d, want 2 (expression pause -> default 0, attacker not frozen)", v)
	}
}

func TestTick_LandedHit_WithoutTheGetHitState_FreezesTheDefenderInPlaceAndDoesNotFail(t *testing.T) {
	cases := []struct {
		name      string
		stateType cns.StateType
		drop      bool
	}{
		{"character with no 5000 state", cns.StateTypeStanding, true},
		{"liedown defender", cns.StateTypeLiedown, false},
		{"undefined-stance defender", cns.StateType("U"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newHitScenario(t, func(a, d map[int]cns.StateDef) {
				def := d[0]
				def.Type = c.stateType
				d[0] = def
				if c.drop {
					delete(d, 5000)
				}
			})
			s.step()
			d := s.ctx(match.SideP2)
			if d.StateNo != 0 || !d.GetHit.Active || d.GetHit.HitShakeTime != 8 {
				t.Fatalf("defender StateNo=%d GetHit=%+v, want state 0 kept with the get-hit vars applied", d.StateNo, d.GetHit)
			}
			counter := d.Vars[1]
			for tick := 2; tick <= 9; tick++ {
				s.step()
				if got := s.ctx(match.SideP2).Vars[1]; got != counter {
					t.Fatalf("tick %d: defender var(1) = %d, want held at %d (frozen)", tick, got, counter)
				}
			}
			s.step()
			if got := s.ctx(match.SideP2).Vars[1]; got != counter+1 {
				t.Errorf("tick 10: defender var(1) = %d, want %d (unfrozen)", got, counter+1)
			}
		})
	}
}

// ticksAllocs measures Tick's allocations per call on the same inputs.
func ticksAllocs(s *hitScenario) float64 {
	tick := 1
	return testing.AllocsPerRun(100, func() {
		cfg := TickConfig{Bounds: testBounds(), Gravity: 0, Tick: tick, ComboWindow: 60}
		if _, err := Tick(s.state, s.programs, s.runtimes, [2]input.TickInput{}, cfg); err != nil {
			s.t.Fatalf("Tick: %v", err)
		}
		tick++
	})
}

func TestApplyHitReaction_AllocatesNothingOnALandedHit(t *testing.T) {
	s := newHitScenario(t, nil)
	params := s.programs[match.SideP1].States[200].Controllers[len(s.programs[match.SideP1].States[200].Controllers)-1].Parameters
	hit := combatHit(match.SideP2)
	allocs := testing.AllocsPerRun(100, func() {
		rts := s.runtimes
		applyHitReaction(&s.programs, &rts, match.SideP1, params, hit)
	})
	if allocs != 0 {
		t.Errorf("applyHitReaction allocated %v times on a landed hit, want 0", allocs)
	}
	rts := s.runtimes
	applyHitReaction(&s.programs, &rts, match.SideP1, params, hit)
	if rts[match.SideP2].Context.StateNo != 5000 {
		t.Fatalf("test setup: the measured call did not enter the get-hit state (StateNo %d)", rts[match.SideP2].Context.StateNo)
	}
}

func TestTick_FrozenTicks_AddNoAllocationBeyondAnIdleTick(t *testing.T) {
	frozen := newHitScenario(t, nil)
	frozen.step() // the hit lands; the runtimes now held are both frozen

	// The idle baseline: the same fighters, but P2 out of range so nothing
	// lands and nobody is frozen. Its allocations are input.Step's
	// per-fighter maps plus whatever the controllers cost.
	idle := newHitScenario(t, nil)
	idle.state.Fighters[match.SideP2].Position.X = 500
	idle.step()
	if idle.ctx(match.SideP2).GetHit.Active || idle.runtimes[match.SideP1].HitPause != 0 {
		t.Fatalf("test setup: the idle scenario's hit landed")
	}

	frozenAllocs, idleAllocs := ticksAllocs(frozen), ticksAllocs(idle)
	if frozenAllocs > idleAllocs {
		t.Errorf("frozen tick allocated %v times, idle tick %v: freezing must add nothing", frozenAllocs, idleAllocs)
	}
}

func TestTick_AHitDefThatFiresWithoutLanding_FreezesNobodyAndChangesNoState(t *testing.T) {
	s := newHitScenario(t, nil)
	s.state.Fighters[match.SideP2].Position.X = 500 // far out of the attack's reach

	s.step() // tick 1: P1's HitDef fires at Time 0, but reaches nothing
	if got := s.runtimes[match.SideP1].HitPause; got != 0 {
		t.Errorf("attacker HitPause = %d, want 0 (nothing landed)", got)
	}
	if got := s.ctx(match.SideP2).GetHit; got != (evaluator.GetHit{}) {
		t.Errorf("defender GetHit = %+v, want the zero value", got)
	}
	if a, d := s.ctx(match.SideP1).StateNo, s.ctx(match.SideP2).StateNo; a != 200 || d != 0 {
		t.Errorf("StateNo attacker/defender = %d/%d, want 200/0 (unchanged)", a, d)
	}
	s.step() // tick 2: every controller runs again
	if got := s.ctx(match.SideP1).Vars[0]; got != 2 {
		t.Errorf("tick 2: attacker var(0) = %d, want 2 (controllers ran)", got)
	}
	if got := s.ctx(match.SideP2).Vars[1]; got != 2 {
		t.Errorf("tick 2: defender var(1) = %d, want 2 (controllers ran)", got)
	}
}

func TestTick_InputKeepsBeingReadWhileFrozen_AndTheComboSurvivesTheFreeze(t *testing.T) {
	s := newHitScenario(t, nil)
	s.programs[match.SideP2].Commands = cmd.CommandFile{
		Defaults: cmd.CommandDefaults{Time: 15, BufferTime: 1},
		Commands: []cmd.Command{{Name: "atk", Input: "a"}},
	}
	s.step() // tick 1: the hit lands
	if got := s.runtimes[match.SideP1].Combo.Count; got != 1 {
		t.Fatalf("after the landing tick: combo count = %d, want 1", got)
	}

	s.in[match.SideP2] = input.TickInput{Buttons: map[string]bool{"a": true}}
	s.step() // tick 2: frozen, the button is held
	if s.ctx(match.SideP2).Vars[1] != 1 {
		t.Fatalf("tick 2: defender controllers ran while frozen")
	}
	if !s.ctx(match.SideP2).ActiveCommands["atk"] {
		t.Errorf("tick 2 (frozen): command atk not active, want it recognised from the held button")
	}
	s.in[match.SideP2] = input.TickInput{}
	s.step() // tick 3: frozen, released
	s.step() // tick 4: frozen, the 1-tick buffer has lapsed
	if s.ctx(match.SideP2).ActiveCommands["atk"] {
		t.Errorf("tick 4 (frozen): command atk still active after the button was released")
	}
	if got := s.runtimes[match.SideP1].Combo.Count; got != 1 {
		t.Errorf("combo count after frozen ticks = %d, want 1 (freeze must not reset it)", got)
	}
}

func TestTick_GroundSlideTime_DrivesAGetHitStateControllerThroughGetHitVar(t *testing.T) {
	// common1_gethit.cns state 5001 sets var(5) once Time >= GetHitVar(slidetime),
	// as Ikemen's IkSys_GetHitKnockedBack applies its velocity multiplier.
	for _, slide := range []string{"5", "2"} {
		t.Run("ground.slidetime = "+slide, func(t *testing.T) {
			want, _ := strconv.Atoi(slide)
			s := newHitScenario(t, func(a, d map[int]cns.StateDef) { setHitDefParam(a, "ground.slidetime", slide) })
			s.step() // tick 1: lands
			for tick := 2; tick <= 10; tick++ {
				s.step() // frozen (2..9), then the 5000 -> 5001 change on tick 10
			}
			// 5001's controllers first run on tick 10, the change tick, with
			// Time = 0 (item 064), so the marker is set on the tick where
			// Time = slidetime.
			for n := 1; n <= want+1; n++ {
				s.step() // tick 10 + n sees Time = n
				flipped := s.ctx(match.SideP2).Vars[5] == 1
				if flipped != (n >= want) {
					t.Fatalf("Time %d (slidetime %d): marker var(5) set = %v, want %v", n, want, flipped, n >= want)
				}
			}
		})
	}
}

func TestTick_Trade_EachFighterGetsItsOwnPauseTimeAndGetHitState(t *testing.T) {
	// Both fighters are in state 200 and land a HitDef the same tick, with
	// different pause times. Tick applies P1's reaction, then P2's; neither
	// overwrites the other's counters. Ordering vs Ikemen GO is unverified.
	s := newHitScenario(t, func(a, d map[int]cns.StateDef) {
		for k, v := range parseStatesFiles(t, "common1_gethit.cns") {
			a[k] = v
		}
		setHitDefParam(a, "pausetime", "3, 6")
		def := a[200]
		def.Anim = 201 // has a hurtbox too, so the attacker can be hit back
		a[200] = def
		d[200] = def
		setHitDefParam(d, "pausetime", "7, 2")
		// Both at 0 and 5: push would part them before the hits land.
		withoutPush(a)
		withoutPush(d)
	})
	s.state.Fighters[match.SideP2].StateNo = 200
	r2, err := NewFighterRuntime(s.state.Fighter(match.SideP2), s.programs[match.SideP2].States)
	if err != nil {
		t.Fatalf("NewFighterRuntime(p2): %v", err)
	}
	s.runtimes[match.SideP2] = r2

	s.step()
	p1, p2 := s.runtimes[match.SideP1], s.runtimes[match.SideP2]
	if p1.HitPause != 3 || p1.Context.GetHit.HitShakeTime != 2 || p1.Context.StateNo != 5000 {
		t.Errorf("P1 HitPause/HitShakeTime/StateNo = %d/%d/%d, want 3/2/5000", p1.HitPause, p1.Context.GetHit.HitShakeTime, p1.Context.StateNo)
	}
	if p2.HitPause != 7 || p2.Context.GetHit.HitShakeTime != 6 || p2.Context.StateNo != 5000 {
		t.Errorf("P2 HitPause/HitShakeTime/StateNo = %d/%d/%d, want 7/6/5000", p2.HitPause, p2.Context.GetHit.HitShakeTime, p2.Context.StateNo)
	}
}

func TestTick_LandedHit_WithKfmTwoValueDamage_TakesTheHitDamageOffTheDefender(t *testing.T) {
	s := newHitScenario(t, nil) // kfm_hitdef.cns: "damage = 23, 0"

	res := s.step()

	if got := res.State.Fighter(match.SideP2).Health; got != 1000-23 {
		t.Fatalf("defender health = %d, want %d (kfm HitDef damage = 23, 0)", got, 1000-23)
	}
}
