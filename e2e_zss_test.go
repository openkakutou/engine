package engine

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openkakutou/character"
	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
)

// End-to-end scenarios (CLAUDE.md, "End-to-end tests (MUST)"): Kung Fu Man
// plays a scripted match through Tick, frame by frame, on the real reference
// files -- the .zss version (kfm_zss with Ikemen GO's common1.cns.zss) and
// the .cns version (kfm with MUGEN's common1.cns). No wall clock, no
// randomness beyond the match's fixed seed. The opponent is a hand-built
// idle dummy far away: the scenarios observe P1.
//
// Expected values come from the files and from Ikemen GO's source (commit
// 54d4148a), cited where used.

// e2eMatchConfig mirrors the conformance mirror match: stage bounds +-500,
// kfm's own yaccel (kfm.cns:76, kfm_zss/kfm.const), a 140-unit start.
const (
	e2eP1X     = -70
	e2eGravity = 0.44
)

// loadReferenceProgram loads a reference character through LoadFighterProgram,
// with testdata/reference standing for Ikemen GO's data/ (src/compiler.go:8793,
// LoadFile dirs {def, "", motif, "data/"}) where stcommon lives.
func loadReferenceProgram(t *testing.T, defPath string) FighterProgram {
	t.Helper()
	path := filepath.Join(referenceDir, defPath)
	prog, missing, err := LoadFighterProgram(loadChar(t, path), path, referenceDir)
	if err != nil {
		t.Fatalf("LoadFighterProgram(%s): %v", defPath, err)
	}
	if len(missing) != 0 {
		t.Fatalf("LoadFighterProgram(%s): state files not found: %v", defPath, missing)
	}
	return prog
}

// e2eMatch is a scripted match: P1 plays prog against an idle dummy.
type e2eMatch struct {
	programs [2]FighterProgram
	state    match.MatchState
	runtimes [2]FighterRuntime
	tick     int
}

func newE2EMatch(t *testing.T, prog FighterProgram) *e2eMatch {
	t.Helper()
	p1 := match.FighterState{Side: match.SideP1, Position: match.Position{X: e2eP1X}, Facing: match.FacingRight, Health: 1000}
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: 900}, Facing: match.FacingLeft, Health: 1000}
	dummy := FighterProgram{States: idleStates()}
	rt1, err := prog.NewRuntime(p1)
	if err != nil {
		t.Fatalf("NewRuntime(P1): %v", err)
	}
	rt2, err := dummy.NewRuntime(p2)
	if err != nil {
		t.Fatalf("NewRuntime(P2): %v", err)
	}
	// Ikemen GO gives every fighter control when the round starts
	// (src/system.go:3364, setSCF(SCF_ctrl, true) as intro reaches 0).
	rt1.Context.Ctrl = true
	state, err := match.NewMatchState(1, 0, p1, p2)
	if err != nil {
		t.Fatalf("NewMatchState: %v", err)
	}
	state.RandSeed = 1
	return &e2eMatch{
		programs: [2]FighterProgram{prog, dummy},
		state:    *state,
		runtimes: [2]FighterRuntime{rt1, rt2},
	}
}

// step runs one tick with P1 holding in.
func (m *e2eMatch) step(in input.TickInput) error {
	m.tick++
	result, err := Tick(m.state, m.programs, m.runtimes, [2]input.TickInput{match.SideP1: in},
		TickConfig{Bounds: testBounds(), Gravity: e2eGravity, Tick: m.tick, ComboWindow: 60})
	if err != nil {
		return fmt.Errorf("tick %d: %w", m.tick, err)
	}
	m.state, m.runtimes = result.State, result.Fighters
	return nil
}

func (m *e2eMatch) p1() (stateNo int, pos match.Position, anim int) {
	ctx := m.runtimes[match.SideP1].Context
	return m.state.Fighters[match.SideP1].StateNo, m.state.Fighters[match.SideP1].Position, ctx.Anim
}

// knownCause is a failure a scenario is known to end in, and the backlog item
// that removes it.
type knownCause struct {
	item   string
	needle string // a substring of the error the cause produces
}

// classifyFailure decides what a scenario's outcome means under an
// expected-failure mark: "passed" when it passed (the mark must go), "known"
// when it failed with a known cause (named in the returned text), "unknown"
// otherwise.
func classifyFailure(causes []knownCause, err error) (outcome, text string) {
	if err == nil {
		items := make([]string, len(causes))
		for i, c := range causes {
			items[i] = c.item
		}
		return "passed", fmt.Sprintf("items %s may now pass: remove the expected-failure mark", strings.Join(items, ", "))
	}
	for _, c := range causes {
		if strings.Contains(err.Error(), c.needle) {
			return "known", fmt.Sprintf("expected failure until item %s (%q): %v", c.item, c.needle, err)
		}
	}
	return "unknown", fmt.Sprintf("scenario failed for a reason no known item covers: %v", err)
}

// expectFailure runs a scenario that cannot pass until the backlog items of
// its causes land: a known cause skips the test naming the item, a pass
// fails it (so the mark cannot rot), any other failure fails it. The
// scenario is never deleted or weakened (CLAUDE.md, "End-to-end tests").
func expectFailure(t *testing.T, causes []knownCause, scenario func() error) {
	t.Helper()
	outcome, text := classifyFailure(causes, scenario())
	if outcome == "known" {
		t.Skip(text)
	}
	t.Fatal(text)
}

func approx(got, want float64) bool { return math.Abs(got-want) < 0.01 }

// scenarioStandIdle: nothing held, the fighter stays in state 0, in place.
func scenarioStandIdle(t *testing.T, prog FighterProgram) error {
	m := newE2EMatch(t, prog)
	for i := 1; i <= 20; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			return err
		}
		stateNo, pos, anim := m.p1()
		if stateNo != 0 || pos.X != e2eP1X || pos.Y != 0 || anim != 0 {
			return fmt.Errorf("tick %d: StateNo, X, Y, Anim = %d, %v, %v, %d, want 0, %d, 0, 0", i, stateNo, pos.X, pos.Y, anim, e2eP1X)
		}
	}
	return nil
}

// scenarioWalkForward: holding forward from idle enters state 20 (walk) and
// moves the fighter at velocity.walk.fwd = 2.4 per tick (kfm.cns:53; a
// walking state's velSet{x: const(velocity.walk.fwd.x)}, common1.cns.zss:76-80).
func scenarioWalkForward(t *testing.T, prog FighterProgram) error {
	m := newE2EMatch(t, prog)
	for i := 0; i < 5; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			return err
		}
	}
	_, before, _ := m.p1()
	const held = 10
	for i := 1; i <= held; i++ {
		if err := m.step(input.TickInput{Right: true}); err != nil {
			return err
		}
	}
	stateNo, after, anim := m.p1()
	if stateNo != 20 {
		return fmt.Errorf("after holding forward for %d ticks StateNo = %d, want 20 (walk)", held, stateNo)
	}
	if anim != 20 {
		return fmt.Errorf("walk Anim = %d, want 20 (walk forward animation, common1 state 20)", anim)
	}
	if want := before.X + held*2.4; !approx(after.X, want) {
		return fmt.Errorf("X = %v after %d walking ticks, want %v (2.4 per tick)", after.X, held, want)
	}
	return nil
}

// scenarioJump: pressing up (one tick) from idle runs jump start (state 40),
// jump up (state 50) with velocity.jump.neu y = -8.4, i.e. 8.4 up
// (kfm.cns:57), under yaccel .44 (kfm.cns:76), then lands (Ikemen GO
// char.go:12584-12590 changes to state 52) and returns to state 0 with Y back
// to 0 (common1.cns.zss:163-178).
func scenarioJump(t *testing.T, prog FighterProgram) error {
	m := newE2EMatch(t, prog)
	if err := m.step(input.TickInput{Up: true}); err != nil {
		return err
	}
	if stateNo, _, _ := m.p1(); stateNo != 40 {
		return fmt.Errorf("after pressing up StateNo = %d, want 40 (jump start)", stateNo)
	}
	sawJumpUp, peak := false, 0.0
	for i := 2; i <= 200; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			return err
		}
		stateNo, pos, _ := m.p1()
		sawJumpUp = sawJumpUp || stateNo == 50
		peak = math.Max(peak, pos.Y)
		if stateNo == 0 && sawJumpUp {
			// A jump of vy0 = 8.4 up under g = .44 peaks near vy0^2 / 2g = 80
			// (the engine keeps Y up, decision 020; Ikemen GO's content has Y down).
			if peak < 70 || peak > 90 {
				return fmt.Errorf("jump peaked at Y = %v, want about 80", peak)
			}
			if pos.Y != 0 {
				return fmt.Errorf("landed in state 0 at Y = %v, want 0", pos.Y)
			}
			return nil
		}
	}
	stateNo, _, _ := m.p1()
	return fmt.Errorf("never back in state 0 after the jump (StateNo %d, saw jump up: %v, peak Y %v)", stateNo, sawJumpUp, peak)
}

// What stops the scenarios today, first cause met on the way in (each is a
// trigger or grammar Ikemen GO has and the engine does not; checked by
// running): both characters' state -1 reach an interval first, `stateNo =
// [200, 299]` (kfm_zss's Function Combo, kfm's state -1 controller 3): item
// 041 (expression grammar). Past it comes `moveContact`: item 042 (its animation family, AnimElem and
// the like, landed first and is not a blocker any more). kfm_zss's
// first trigger, `roundState = 2 && !aiLevelF`, evaluates since item 042's
// "Other" family (AILevel, Const, FVar, PalNo, NumHelper) landed.
var (
	zssCauses = []knownCause{
		{"041 (intervals)", "unexpected character '['"},
		{"042 (moveContact)", `unknown trigger "movecontact"`},
	}
	cnsCauses = []knownCause{
		{"041 (intervals)", "unexpected character '['"},
		{"042 (moveContact)", `unknown trigger "movecontact"`},
	}
)

// landingCause stops the jump once the triggers above are there: the engine
// does not change a fighter in physics type A to state 52 when it lands
// (Ikemen GO char.go:12584-12590), so a jump never returns to state 0. No
// backlog item covers it yet; item 044 (physics order) is the closest home.
var landingCause = knownCause{"none yet: landing changes to state 52 (propose adding it to 044)", "never back in state 0"}

const (
	kfmZssDef = "chars/kfm_zss/kfm_zss.def"
	kfmCnsDef = "chars/kfm/kfm.def"
)

func TestE2E_KfmZss_StandIdle_StaysInStateZero(t *testing.T) {
	prog := loadReferenceProgram(t, kfmZssDef)
	expectFailure(t, zssCauses, func() error { return scenarioStandIdle(t, prog) })
}

func TestE2E_KfmZss_HoldForward_WalksAtTheCharactersWalkSpeed(t *testing.T) {
	prog := loadReferenceProgram(t, kfmZssDef)
	expectFailure(t, zssCauses, func() error { return scenarioWalkForward(t, prog) })
}

func TestE2E_KfmZss_PressUp_JumpsAndLandsInStateZero(t *testing.T) {
	prog := loadReferenceProgram(t, kfmZssDef)
	expectFailure(t, append(zssCauses[:len(zssCauses):len(zssCauses)], landingCause), func() error { return scenarioJump(t, prog) })
}

func TestE2E_KfmCns_StandIdle_StaysInStateZero(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	expectFailure(t, cnsCauses, func() error { return scenarioStandIdle(t, prog) })
}

func TestE2E_KfmCns_HoldForward_WalksAtTheCharactersWalkSpeed(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	expectFailure(t, cnsCauses, func() error { return scenarioWalkForward(t, prog) })
}

func TestE2E_KfmCns_PressUp_JumpsAndLandsInStateZero(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	expectFailure(t, append(cnsCauses[:len(cnsCauses):len(cnsCauses)], landingCause), func() error { return scenarioJump(t, prog) })
}

func TestClassifyFailure_PassingScenarioUnknownAndKnownCause_AreToldApart(t *testing.T) {
	causes := []knownCause{{"041", "bracket"}, {"042", "trigger x"}}
	for _, tc := range []struct {
		name        string
		err         error
		wantOutcome string
		wantText    string
	}{
		{"passes", nil, "passed", "items 041, 042 may now pass"},
		{"known cause", fmt.Errorf("tick 1: unknown trigger x"), "known", "until item 042"},
		{"another cause", fmt.Errorf("tick 1: boom"), "unknown", "no known item"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcome, text := classifyFailure(causes, tc.err)
			if outcome != tc.wantOutcome || !strings.Contains(text, tc.wantText) {
				t.Errorf("classifyFailure = %q, %q, want %q mentioning %q", outcome, text, tc.wantOutcome, tc.wantText)
			}
		})
	}
}

// miniZss is a few states written like common1.cns.zss's own (state 0, walk
// 20, jump 40/50/52) driven by a state -1 on the real kfm_zss commands: the
// scenarios above must pass on a character the engine can run, or they would
// assert nothing. The landing in state 50 is spelled out because the engine
// does not do Ikemen GO's change to state 52 on landing yet (see jumpCauses).
const miniZss = `
[StateDef 0; type: S; physics: S;]
velSet{x: 0}

[StateDef -1]
if command = "holdup" && stateNo = 0 && ctrl {
	changeState{value: 40}
}
if command = "holdfwd" && stateNo = 0 && ctrl {
	changeState{value: 20}
}

[StateDef 20; type: S; physics: S;]
velSet{x: 2.4}
changeAnim{value: 20}

[StateDef 40; type: S; physics: S; anim: 40; ctrl: 0;]
if time = 2 {
	velSet{x: 0; y: -8.4}
	changeState{value: 50}
}

[StateDef 50; type: A; physics: A;]
if time > 0 && pos y >= 0 {
	changeState{value: 52}
}

[StateDef 52; type: S; physics: S; anim: 47; ctrl: 0;]
velSet{y: 0}
if time = 3 {
	changeState{value: 0}
}
`

func miniZssProgram(t *testing.T) FighterProgram {
	t.Helper()
	ch, err := character.Load(filepath.Join(referenceDir, kfmZssDef))
	if err != nil {
		t.Fatalf("character.Load: %v", err)
	}
	return NewFighterProgram(nil, nil, ch.Commands).WithScripts([]zss.Script{mustScript(t, miniZss)})
}

func TestE2EScenarios_MiniZssCharacterOnRealCommands_Pass(t *testing.T) {
	prog := miniZssProgram(t)
	for name, scenario := range map[string]func(*testing.T, FighterProgram) error{
		"stand idle":   scenarioStandIdle,
		"walk forward": scenarioWalkForward,
		"jump":         scenarioJump,
	} {
		t.Run(name, func(t *testing.T) {
			if err := scenario(t, prog); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestE2EScenarios_WalkAndJump_FailOnACharacterThatDoesNotMove(t *testing.T) {
	// A scenario that no wrong engine could fail is worth nothing: a
	// character that only stands (state 0) must fail walk and jump.
	ch, err := character.Load(filepath.Join(referenceDir, kfmZssDef))
	if err != nil {
		t.Fatalf("character.Load: %v", err)
	}
	prog := NewFighterProgram(nil, nil, ch.Commands).WithScripts([]zss.Script{mustScript(t, "[StateDef 0; type: S; physics: S;]\nnull{}")})
	if err := scenarioWalkForward(t, prog); err == nil || !strings.Contains(err.Error(), "StateNo = 0, want 20") {
		t.Errorf("walk scenario on a standing-only character: %v, want a StateNo failure", err)
	}
	if err := scenarioJump(t, prog); err == nil || !strings.Contains(err.Error(), "want 40") {
		t.Errorf("jump scenario on a standing-only character: %v, want a StateNo failure", err)
	}
}
