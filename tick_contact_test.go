package engine

import (
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// Item 042, last slice: what a fighter sees of its opponent within a tick, and
// the hit-contact triggers. Expected values follow Ikemen GO's source (commit
// 54d4148a), cited per test.

func recordVar(slot, value string, triggers ...string) cns.Controller {
	if len(triggers) == 0 {
		triggers = []string{"1"}
	}
	return cns.Controller{Type: "VarSet", Triggers: triggers, Parameters: map[string]string{"v": slot, "value": value}}
}

func changeTo(state string, triggers ...string) cns.Controller {
	return cns.Controller{Type: "ChangeState", Triggers: triggers, Parameters: map[string]string{"value": state}}
}

func standing(number int, ctrls ...cns.Controller) cns.StateDef {
	return cns.StateDef{Number: number, Type: cns.StateTypeStanding, Ctrl: true, Controllers: ctrls}
}

// Ikemen GO runs the fighters one after the other inside charList.action
// (char.go:14066-14090): fighter 2 sees what fighter 1's controllers did in
// this very tick (its new state number), fighter 1 sees fighter 2 as the
// previous tick left it. Neither moves here; fighter 2 reading fighter 1's
// movement is TestTick_SecondFighterSeesTheFirstOnesPositionAfterItsMovement.
func TestTick_OpponentTriggers_SecondFighterSeesTheFirstOnesNewState_FirstSeesTheSecondsPreviousTick(t *testing.T) {
	p1States := map[int]cns.StateDef{
		0:   standing(0, recordVar("0", "P2StateNo"), recordVar("2", "P2Dist X"), changeTo("100", "Time = 0")),
		100: standing(100, recordVar("1", "P2StateNo")),
	}
	p2States := map[int]cns.StateDef{
		0:   standing(0, recordVar("0", "P2StateNo"), recordVar("2", "P2Dist X"), changeTo("300", "Time = 0")),
		300: standing(300, recordVar("1", "P2StateNo")),
	}
	p1 := match.FighterState{Side: match.SideP1, Facing: match.FacingRight, Position: match.Position{X: -10}, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Facing: match.FacingLeft, Position: match.Position{X: 30}, Health: 1000}
	r1, err := NewFighterRuntime(p1, p1States)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := NewFighterRuntime(p2, p2States)
	if err != nil {
		t.Fatal(err)
	}
	st, err := match.NewMatchState(1, 1000, p1, p2)
	if err != nil {
		t.Fatal(err)
	}
	progs := [2]FighterProgram{{States: p1States}, {States: p2States}}
	runtimes := [2]FighterRuntime{r1, r2}
	for tick := 1; tick <= 2; tick++ {
		res, err := Tick(*st, progs, runtimes, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Tick: tick, ComboWindow: 60})
		if err != nil {
			t.Fatal(err)
		}
		st, runtimes = &res.State, res.Fighters
	}
	c1, c2 := runtimes[match.SideP1].Context, runtimes[match.SideP2].Context
	if c1.Vars[0] != 0 || c2.Vars[0] != 100 {
		t.Errorf("tick 1 P2StateNo: P1 saw %d, P2 saw %d, want 0 (P2 not run yet) and 100 (P1 already changed)", c1.Vars[0], c2.Vars[0])
	}
	if c1.Vars[1] != 300 || c2.Vars[1] != 100 {
		t.Errorf("tick 2 P2StateNo: P1 saw %d, P2 saw %d, want 300 and 100", c1.Vars[1], c2.Vars[1])
	}
	// P2Dist X at tick 1: facing * (opponent X - own X): 40 for P1, and for the
	// left-facing P2 -1 * (-10 - 30) = 40.
	if c1.Vars[2] != 40 || c2.Vars[2] != 40 {
		t.Errorf("tick 1 P2Dist X: P1 saw %d, P2 saw %d, want 40 and 40", c1.Vars[2], c2.Vars[2])
	}
}

// contactScenario is the kfm hit scenario with controllers on the attacker
// recording what it reads each tick: var(2) = MoveContact, var(3) = MoveHit,
// var(4) = 1 while HitDefAttr = SCA, NA.
func contactScenario(t *testing.T, extra ...cns.Controller) *hitScenario {
	t.Helper()
	return newHitScenario(t, func(a, d map[int]cns.StateDef) {
		def := a[200]
		def.Controllers = append([]cns.Controller{
			recordVar("2", "MoveContact"),
			recordVar("3", "MoveHit"),
			recordVar("4", "1", "HitDefAttr = SCA, NA"),
		}, def.Controllers...)
		def.Controllers = append(def.Controllers, extra...)
		a[200] = def
	})
}

// Ikemen GO: the contact sets mctime = -1 in hit detection (char.go:14270-14271)
// and the same tick's tick() turns it into 1 (:13125-13126); the counter then
// grows by one after each unfrozen action (:12594-12596), so the attacker's
// controllers read 1 through the whole hit pause and 2 on the tick after.
func TestTick_LandedHit_AttackerReadsMoveHitAndMoveContactAsIkemenGO(t *testing.T) {
	s := contactScenario(t)
	s.step() // tick 1: the hit lands (pausetime = 8, 8)
	if got := s.ctx(match.SideP1).MoveContactTime; got != 1 {
		t.Fatalf("after the landing tick MoveContactTime = %d, want 1", got)
	}
	if got := s.ctx(match.SideP1).Vars[2]; got != 0 {
		t.Errorf("landing tick: the attacker's own controllers read MoveContact = %d, want 0 (the hit lands after them)", got)
	}
	for tick := 2; tick <= 9; tick++ { // the 8 frozen ticks
		s.step()
		if got := s.ctx(match.SideP1).MoveContactTime; got != 1 {
			t.Fatalf("tick %d: MoveContactTime = %d, want 1 (held through the hit pause)", tick, got)
		}
	}
	s.step() // tick 10: first unfrozen tick, controllers read 1, then it becomes 2
	a := s.ctx(match.SideP1)
	if a.Vars[2] != 1 || a.Vars[3] != 1 || a.MoveContactTime != 2 {
		t.Errorf("tick 10: read MoveContact %d, MoveHit %d, counter after = %d, want 1, 1, 2", a.Vars[2], a.Vars[3], a.MoveContactTime)
	}
	s.step()
	if got := s.ctx(match.SideP1).Vars[2]; got != 2 {
		t.Errorf("tick 11: MoveContact read %d, want 2", got)
	}
	if d := s.ctx(match.SideP2); d.MoveContactTime != 0 {
		t.Errorf("the defender's MoveContactTime = %d, want 0 (it did not hit)", d.MoveContactTime)
	}
}

func TestTick_AttackerWithHitDef_ReadsHitDefAttrOnlyWhileInAnAttackMoveType(t *testing.T) {
	s := contactScenario(t)
	s.step() // tick 1: HitDef runs (attr = S, NA)
	if got := s.ctx(match.SideP1).HitDefAttr; got == 0 {
		t.Fatalf("HitDefAttr after the HitDef ran = 0, want the parsed S, NA")
	}
	for tick := 2; tick <= 10; tick++ {
		s.step()
	}
	if got := s.ctx(match.SideP1).Vars[4]; got != 1 {
		t.Errorf("var(4) = %d, want 1: HitDefAttr = SCA, NA holds in state 200 (movetype A) after the HitDef", got)
	}
}

// Ikemen GO clears the contact counter when a state is entered unless its
// StateDef says movehitpersist (bytecode.go:5456-5459 with the default 0,
// compiler.go:7072-7079).
func TestTick_StateChange_ResetsTheMoveContactCounter_UnlessMoveHitPersist(t *testing.T) {
	for _, persist := range []bool{false, true} {
		s := newHitScenario(t, func(a, d map[int]cns.StateDef) {
			def := a[200]
			def.Controllers = append(def.Controllers, changeTo("210", "MoveContact = 3"))
			a[200] = def
			a[210] = cns.StateDef{Number: 210, Type: cns.StateTypeStanding, MoveType: "A", Ctrl: true, MoveHitPersist: persist}
		})
		for tick := 1; tick <= 12; tick++ { // reads 1 at tick 10, 2 at 11, 3 at 12: the change
			s.step()
		}
		a := s.ctx(match.SideP1)
		if a.StateNo != 210 {
			t.Fatalf("persist=%v: attacker StateNo = %d, want 210", persist, a.StateNo)
		}
		want := 0
		if persist {
			want = 4 // kept, and the tick's own increment follows (char.go:12594-12596)
		}
		if a.MoveContactTime != want {
			t.Errorf("movehitpersist=%v: MoveContactTime after the change = %d, want %d", persist, a.MoveContactTime, want)
		}
	}
}

func TestTick_LandedHit_RecordsTheFallAndVelocityGetHitVars(t *testing.T) {
	cases := []struct {
		name  string
		tweak func(a, d map[int]cns.StateDef)
		want  evaluator.GetHit
	}{
		{"defaults of a standing hit", nil, evaluator.GetHit{
			FallEnvShake: evaluator.EnvShake{Freq: 60, Ampl: -4}, FallKill: true, DownRecoverTime: 60, Active: true, HitShakeTime: 8, HitTime: 11, SlideTime: 5, GroundType: 1, AirType: 1,
			Yaccel: 0.35, FallYvel: -4.5, FallRecover: true, FallRecoverTime: 4, CtrlTime: 11,
		}},
		{"a launching hit on a standing defender", func(a, d map[int]cns.StateDef) {
			setHitDefParam(a, "ground.velocity", "-2.5, -7")
			setHitDefParam(a, "yaccel", ".5")
			setHitDefParam(a, "fall", "1")
			setHitDefParam(a, "fall.yvelocity", "-5.5")
			setHitDefParam(a, "fall.recover", "0")
			setHitDefParam(a, "fall.recovertime", "7")
			setHitDefParam(a, "id", "4")
		}, evaluator.GetHit{
			FallEnvShake: evaluator.EnvShake{Freq: 60, Ampl: -4}, FallKill: true, DownRecoverTime: 60, Active: true, HitShakeTime: 8, HitTime: 11, SlideTime: 5, AnimType: 3, GroundType: 1, AirType: 1,
			Yvel: -7, Yaccel: 0.5, Fall: true, FallYvel: -5.5, FallRecoverTime: 7, CtrlTime: 11, HitID: 4,
			Xvel: 2.5, // -2.5 from an attacker facing right pushes right, away (char.go:11793)
		}},
		// char.go:11796-11799: a falling ground hit with no vertical velocity
		// gets -0.001 "as some form of internal workaround".
		{"a falling hit without a y velocity", func(a, d map[int]cns.StateDef) {
			setHitDefParam(a, "fall", "1")
		}, evaluator.GetHit{
			FallEnvShake: evaluator.EnvShake{Freq: 60, Ampl: -4}, FallKill: true, DownRecoverTime: 60, Active: true, HitShakeTime: 8, HitTime: 11, SlideTime: 5, AnimType: 3, GroundType: 1, AirType: 1,
			Yvel: -0.001, Yaccel: 0.35, Fall: true, FallYvel: -4.5, FallRecover: true, FallRecoverTime: 4, CtrlTime: 11,
		}},
		// char.go:8102-8113 (gethitAnimtype): a ground Back/Up/DiagUp hit that
		// does not lift the defender reads as Hard; with the fall flag the
		// fall.animtype decides, which defaults to Back unless air.animtype is
		// Up or DiagUp (char.go:958-967).
		{"a Back animtype without lift reads Hard", func(a, d map[int]cns.StateDef) {
			setHitDefParam(a, "animtype", "Back")
		}, evaluator.GetHit{
			FallEnvShake: evaluator.EnvShake{Freq: 60, Ampl: -4}, FallKill: true, DownRecoverTime: 60, Active: true, HitShakeTime: 8, HitTime: 11, SlideTime: 5, AnimType: 2, GroundType: 1, AirType: 1,
			Yaccel: 0.35, FallYvel: -4.5, FallRecover: true, FallRecoverTime: 4, CtrlTime: 11,
		}},
		{"a falling hit reads its fall.animtype", func(a, d map[int]cns.StateDef) {
			setHitDefParam(a, "animtype", "Light")
			setHitDefParam(a, "fall", "1")
			setHitDefParam(a, "fall.animtype", "Up")
		}, evaluator.GetHit{
			FallEnvShake: evaluator.EnvShake{Freq: 60, Ampl: -4}, FallKill: true, DownRecoverTime: 60, Active: true, HitShakeTime: 8, HitTime: 11, SlideTime: 5, AnimType: 4, GroundType: 1, AirType: 1,
			Yvel: -0.001, Yaccel: 0.35, Fall: true, FallYvel: -4.5, FallRecover: true, FallRecoverTime: 4, CtrlTime: 11,
		}},
		{"a falling hit defaults fall.animtype to Back", func(a, d map[int]cns.StateDef) {
			setHitDefParam(a, "fall", "1")
		}, evaluator.GetHit{
			FallEnvShake: evaluator.EnvShake{Freq: 60, Ampl: -4}, FallKill: true, DownRecoverTime: 60, Active: true, HitShakeTime: 8, HitTime: 11, SlideTime: 5, AnimType: 3, GroundType: 1, AirType: 1,
			Yvel: -0.001, Yaccel: 0.35, Fall: true, FallYvel: -4.5, FallRecover: true, FallRecoverTime: 4, CtrlTime: 11,
		}},
		// char.go:11763-11772: an airborne defender takes air.velocity, air.fall
		// and air.hittime for its control time.
		{"an airborne defender", func(a, d map[int]cns.StateDef) {
			def := d[0]
			def.Type = cns.StateTypeAir
			d[0] = def
			setHitDefParam(a, "air.velocity", "-1, -9.25")
			setHitDefParam(a, "fall", "1")
			setHitDefParam(a, "air.fall", "0")
		}, evaluator.GetHit{
			FallEnvShake: evaluator.EnvShake{Freq: 60, Ampl: -4}, FallKill: true, DownRecoverTime: 60, Active: true, HitShakeTime: 8, HitTime: 15, SlideTime: 5, GroundType: 1, AirType: 1,
			Yvel: -9.25, Yaccel: 0.35, FallYvel: -4.5, FallRecover: true, FallRecoverTime: 4, CtrlTime: 15,
			Xvel: 1, // air.velocity x -1, attacker facing right
		}},
	}
	for _, tc := range cases {
		s := newHitScenario(t, tc.tweak)
		s.step()
		// Every case is a first hit of 23 (ghv.hitcount, ghv.damage).
		tc.want.HitCount, tc.want.Damage = 1, 23
		if got := s.ctx(match.SideP2).GetHit; got != tc.want {
			t.Errorf("%s: defender GetHit = %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// char.go:12648-12657 steps fall_time on every tick the fighter is in a hit
// state with the fall flag (even during the shake), and :12672 clears the flag
// on the first tick it is out of one; canRecover is fall_recover && fall_time
// >= fall_recovertime (:5660).
func TestTick_FallingDefender_CountsFallTimeInTheHitStateAndClearsTheFlagOnRecovery(t *testing.T) {
	s := newHitScenario(t, func(a, d map[int]cns.StateDef) { setHitDefParam(a, "fall", "1") })
	s.step() // lands
	if g := s.ctx(match.SideP2).GetHit; !g.Fall || g.FallTime != 0 {
		t.Fatalf("after landing: Fall, FallTime = %v, %d, want true, 0", g.Fall, g.FallTime)
	}
	for tick := 2; tick <= 5; tick++ {
		s.step()
		if got := s.ctx(match.SideP2).GetHit.FallTime; got != tick-1 {
			t.Fatalf("tick %d: FallTime = %d, want %d", tick, got, tick-1)
		}
	}
	if !evalBool(t, "CanRecover", s.ctx(match.SideP2)) {
		t.Errorf("CanRecover at FallTime 4 = false, want true (fall.recovertime defaults to 4)")
	}
	for tick := 6; tick <= 60 && s.ctx(match.SideP2).StateNo != 0; tick++ {
		s.step()
	}
	d := s.ctx(match.SideP2)
	if d.StateNo != 0 || d.GetHit.Fall {
		t.Errorf("after recovering: StateNo = %d, Fall = %v, want 0, false", d.StateNo, d.GetHit.Fall)
	}
}

func evalBool(t *testing.T, expr string, ctx evaluator.Context) bool {
	t.Helper()
	v, err := evaluator.Evaluate(expr, ctx)
	if err != nil {
		t.Fatalf("Evaluate(%q): %v", expr, err)
	}
	return v.Bool()
}

func TestTick_LandedHit_MakesTheDefenderTheAttackersTargetWhileItIsInAHitState(t *testing.T) {
	s := newHitScenario(t, nil)
	s.step()
	if !s.ctx(match.SideP1).HasTarget {
		t.Fatal("the attacker has no target after landing a hit")
	}
	s.step() // P1 is frozen; P2 is in 5000 (movetype H)
	if got := evalBool(t, "NumTarget", s.ctx(match.SideP1)); !got {
		t.Errorf("NumTarget during the hit = 0, want 1")
	}
	for tick := 3; tick <= 60 && s.ctx(match.SideP2).StateNo != 0; tick++ {
		s.step()
	}
	s.step()
	if s.ctx(match.SideP1).HasTarget {
		t.Errorf("the attacker still has a target after the defender left its hit states (char.go:10387-10404)")
	}
}

// tickAllocs is Tick's allocations per warm call with every fighter running
// ctrls in state 0.
func tickAllocs(t *testing.T, ctrls []cns.Controller) float64 {
	t.Helper()
	states := map[int]cns.StateDef{0: standing(0, ctrls...)}
	p1 := match.FighterState{Side: match.SideP1, Facing: match.FacingRight, Position: match.Position{X: -10}, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Facing: match.FacingLeft, Position: match.Position{X: 10}, Health: 1000}
	rt1, err := NewFighterRuntime(p1, states)
	if err != nil {
		t.Fatal(err)
	}
	rt2, err := NewFighterRuntime(p2, states)
	if err != nil {
		t.Fatal(err)
	}
	st, err := match.NewMatchState(1, 1000, p1, p2)
	if err != nil {
		t.Fatal(err)
	}
	progs := [2]FighterProgram{{States: states}, {States: states}}
	runtimes := [2]FighterRuntime{rt1, rt2}
	tick := 1
	run := func() {
		if _, err := Tick(*st, progs, runtimes, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Tick: tick, ComboWindow: 60}); err != nil {
			t.Fatalf("Tick: %v", err)
		}
		tick++
	}
	run() // warm the expression caches
	return testing.AllocsPerRun(100, run)
}

// The opponent snapshot is a value in Context, not a pointer, so reading the
// opponent and the contact triggers costs no allocation beyond what the same
// controllers cost with literal values (input recognition and the applied
// controllers' bookkeeping, already there).
func TestTick_OpponentAndContactTriggers_WarmTickAddsNoAllocation(t *testing.T) {
	literal := []cns.Controller{
		recordVar("0", "1"),
		recordVar("1", "1"),
		recordVar("2", "1"),
		recordVar("3", "1", "1", "1", "1"),
	}
	reading := []cns.Controller{
		recordVar("0", "P2BodyDist X + P2Dist Y + P2Life"),
		recordVar("1", "MoveContact + MoveHit + HitFall + NumEnemy + NumTarget"),
		recordVar("2", "p2,time + p2,getHitVar(hittime) + GetHitVar(yvel)"),
		recordVar("3", "1", "!HitDefAttr = SCA, NA", "P2StateType = S", "P2MoveType != H"),
	}
	want, got := tickAllocs(t, literal), tickAllocs(t, reading)
	if got != want {
		t.Errorf("Tick with opponent and contact triggers allocated %v times, want %v (the same controllers with literal values)", got, want)
	}
}
