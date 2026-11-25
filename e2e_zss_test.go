package engine

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openkakutou/character"
	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
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

// e2eP1X mirrors the conformance mirror match: stage bounds +-500, a
// 140-unit start.
const e2eP1X = -70

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
	sounds   [2][]evaluator.SoundEvent // the last tick's sound events
	warnings [2][]string               // the last tick's warnings
}

func newE2EMatch(t *testing.T, prog FighterProgram) *e2eMatch {
	t.Helper()
	return newE2EMatchAt(t, prog, 0)
}

// newE2EMatchAt starts P1 in state stateNo.
func newE2EMatchAt(t *testing.T, prog FighterProgram, stateNo int) *e2eMatch {
	t.Helper()
	p1 := match.FighterState{Side: match.SideP1, Position: match.Position{X: e2eP1X}, Facing: match.FacingRight, Health: 1000, StateNo: stateNo}
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
		TickConfig{Bounds: testBounds(), Tick: m.tick, ComboWindow: 60})
	if err != nil {
		return fmt.Errorf("tick %d: %w", m.tick, err)
	}
	m.state, m.runtimes, m.sounds, m.warnings = result.State, result.Fighters, result.Sounds, result.Warnings
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
// whose last tick sets velocity.jump.neu y = -8.4, i.e. 8.4 up (kfm.cns:57),
// and changes to jump up (state 50, physics A); the fighter then lands (Ikemen
// GO char.go:12584-12590 changes to state 52) and returns to state 0 with Y
// back to 0 (common1.cns:267-297, common1.cns.zss:163-178).
//
// The arc, by hand, in Ikemen GO's order (posUpdate, char.go:10102-10139:
// position += velocity, then velocity += yaccel .44, kfm.cns:76), with n
// counting from the VelSet tick (n = 1, which already moves: state 50 is
// entered before the movement, char.go:12583): Y(n) = 8.4n - 0.22n(n-1) in
// the engine's Y up (decision 020). Y(1) = 8.4; the velocity is still upward
// for the moves k = 0..19 (8.4 - .44 * 19 = .04), so the peak is
// Y(20) = 168 - 83.6 = 84.4; Y(39) = 1.56 and Y(40) = 336 - 343.2 = -7.2,
// falling, so tick n = 40 lands in state 52 at Y 0.
func scenarioJump(t *testing.T, prog FighterProgram) error {
	m := newE2EMatch(t, prog)
	if err := m.step(input.TickInput{Up: true}); err != nil {
		return err
	}
	if stateNo, _, _ := m.p1(); stateNo != 40 {
		return fmt.Errorf("after pressing up StateNo = %d, want 40 (jump start)", stateNo)
	}
	n, peak := 0, 0.0
	for i := 2; i <= 200; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			return err
		}
		stateNo, pos, _ := m.p1()
		if stateNo == 50 || n > 0 {
			n++
		}
		peak = math.Max(peak, pos.Y)
		if n == 1 && !approx(pos.Y, 8.4) {
			return fmt.Errorf("first tick of state 50: Y = %v, want 8.4 (the VelSet moves on its own tick)", pos.Y)
		}
		if n > 0 && n < 40 && stateNo != 50 {
			return fmt.Errorf("jump tick %d: StateNo = %d, want 50 (Y %v)", n, stateNo, pos.Y)
		}
		if n == 40 && (stateNo != 52 || pos.Y != 0) {
			return fmt.Errorf("jump tick 40: StateNo, Y = %d, %v, want 52, 0 (landing)", stateNo, pos.Y)
		}
		if stateNo == 0 && n >= 40 {
			if !approx(peak, 84.4) {
				return fmt.Errorf("jump peaked at Y = %v, want 84.4", peak)
			}
			if pos.Y != 0 {
				return fmt.Errorf("landed in state 0 at Y = %v, want 0", pos.Y)
			}
			return nil
		}
	}
	stateNo, _, _ := m.p1()
	return fmt.Errorf("never back in state 0 after the jump (StateNo %d, jump ticks %d, peak Y %v)", stateNo, n, peak)
}

// scenarioAttackSound: from idle, pressing button for one tick enters the
// attack state stateNo, and over its first five ticks P1's PlaySnd events
// are exactly want. A failure names the first mismatch.
func scenarioAttackSound(t *testing.T, prog FighterProgram, button string, stateNo int, want evaluator.SoundEvent) error {
	m := newE2EMatch(t, prog)
	for i := 0; i < 5; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			return err
		}
	}
	if err := m.step(input.TickInput{Buttons: map[string]bool{button: true}}); err != nil {
		return err
	}
	if got, _, _ := m.p1(); got != stateNo {
		return fmt.Errorf("after pressing %s StateNo = %d, want %d", button, got, stateNo)
	}
	var heard []evaluator.SoundEvent
	for i := 0; i < 5; i++ {
		heard = append(heard, m.sounds[match.SideP1]...)
		if err := m.step(input.TickInput{}); err != nil {
			return err
		}
	}
	if len(heard) != 1 || heard[0] != want {
		return fmt.Errorf("P1 sounds over state %d's first ticks = %v, want exactly [%v]", stateNo, heard, want)
	}
	return nil
}

const (
	kfmZssDef = "chars/kfm_zss/kfm_zss.def"
	kfmCnsDef = "chars/kfm/kfm.def"
)

func TestE2E_KfmZss_StandIdle_StaysInStateZero(t *testing.T) {
	prog := loadReferenceProgram(t, kfmZssDef)
	if err := scenarioStandIdle(t, prog); err != nil {
		t.Fatal(err)
	}
}

func TestE2E_KfmZss_HoldForward_WalksAtTheCharactersWalkSpeed(t *testing.T) {
	prog := loadReferenceProgram(t, kfmZssDef)
	if err := scenarioWalkForward(t, prog); err != nil {
		t.Fatal(err)
	}
}

func TestE2E_KfmZss_PressUp_JumpsAndLandsInStateZero(t *testing.T) {
	prog := loadReferenceProgram(t, kfmZssDef)
	if err := scenarioJump(t, prog); err != nil {
		t.Fatal(err)
	}
}

func TestE2E_KfmCns_StandIdle_StaysInStateZero(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	if err := scenarioStandIdle(t, prog); err != nil {
		t.Fatal(err)
	}
}

func TestE2E_KfmCns_HoldForward_WalksAtTheCharactersWalkSpeed(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	if err := scenarioWalkForward(t, prog); err != nil {
		t.Fatal(err)
	}
}

func TestE2E_KfmCns_PressUp_JumpsAndLandsInStateZero(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	if err := scenarioJump(t, prog); err != nil {
		t.Fatal(err)
	}
}

// Stand light kick, state 230 (command.zss:253-259): at Time = 2 it calls
// CSnd(0, 0, 1) (kfm.zss:339-341), i.e. playSnd{value: S $bank, $sound;
// channel: $c} (kfm.zss:28-29). S reads the character's own sounds (Ikemen
// GO src/char.go:6655-6657), so the event is not common, on channel 1.
func TestE2E_KfmZss_PressA_PlaysTheLightKickSoundFromItsOwnFile(t *testing.T) {
	prog := loadReferenceProgram(t, kfmZssDef)
	if err := scenarioAttackSound(t, prog, "a", 230, evaluator.SoundEvent{Group: 0, Sample: 0, Channel: 1}); err != nil {
		t.Fatal(err)
	}
}

// Stand light punch, state 200 (command.zss:238-243): at Time = 1 it calls
// CSnd(0, 0, 1) (kfm.zss:253-258).
func TestE2E_KfmZss_PressX_PlaysTheLightPunchSoundFromItsOwnFile(t *testing.T) {
	prog := loadReferenceProgram(t, kfmZssDef)
	if err := scenarioAttackSound(t, prog, "x", 200, evaluator.SoundEvent{Group: 0, Sample: 0, Channel: 1}); err != nil {
		t.Fatal(err)
	}
}

// Stand light punch, state 200 (kfm.cmd:630-637): PlaySnd value = 0, 0 at
// Time = 1 (kfm.cns:295-297), no prefix and no channel (-1, Ikemen GO
// newPlaySndParams, src/sound.go:1471).
func TestE2E_KfmCns_PressX_PlaysTheLightPunchSoundFromItsOwnFile(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	if err := scenarioAttackSound(t, prog, "x", 200, evaluator.SoundEvent{Group: 0, Sample: 0, Channel: -1}); err != nil {
		t.Fatal(err)
	}
}

// scenarioLightKickHitsKfm: kfm_zss (P1) presses a next to the .cns kfm (P2)
// standing idle. State 230's hitDef (animElem = 3, kfm.zss:343-359) lands:
// P2 loses its damage, 26, once, and enters a get-hit state (5000-series,
// common1.cns:873; get-hit entry, decision 018). P2 stands 40 units ahead.
func scenarioLightKickHitsKfm(t *testing.T, attacker, defender FighterProgram) error {
	m := newE2EMatch(t, attacker)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 40}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := defender.NewRuntime(p2)
	if err != nil {
		return fmt.Errorf("NewRuntime(P2): %w", err)
	}
	m.programs[match.SideP2] = defender
	m.runtimes[match.SideP2] = rt2
	m.state.Fighters[match.SideP2] = p2
	if err := m.step(input.TickInput{Buttons: map[string]bool{"a": true}}); err != nil {
		return err
	}
	if got, _, _ := m.p1(); got != 230 {
		return fmt.Errorf("after pressing a StateNo = %d, want 230", got)
	}
	for i := 0; i < 30; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			return err
		}
		p2 := m.state.Fighters[match.SideP2]
		if p2.Health == 1000 {
			continue
		}
		if p2.Health != 1000-26 {
			return fmt.Errorf("tick %d: P2 life = %d, want %d (state 230 damage 26)", m.tick, p2.Health, 1000-26)
		}
		if p2.StateNo < 5000 || p2.StateNo > 5999 {
			return fmt.Errorf("tick %d: P2 StateNo = %d on the hit, want a 5000-series get-hit state", m.tick, p2.StateNo)
		}
		return nil
	}
	return fmt.Errorf("P2 life still 1000 after state 230 ran 30 ticks: the light kick never hit")
}

func TestE2E_KfmZssVsKfm_PressA_LightKickDamagesTheOpponent(t *testing.T) {
	if err := scenarioLightKickHitsKfm(t, loadReferenceProgram(t, kfmZssDef), loadReferenceProgram(t, kfmCnsDef)); err != nil {
		t.Fatal(err)
	}
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
// assert nothing. Like common1, state 50 relies on the engine's own landing
// into state 52 (Ikemen GO char.go:12584-12590).
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
null{}

[StateDef 52; type: S; physics: S; anim: 47; ctrl: 0;]
velSet{y: 0}
posSet{y: 0}
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
	// It enters the walk and jump states empty, as Ikemen GO does
	// (char.go:6880-6887): no walk animation, no jump, so both still fail.
	if err := scenarioWalkForward(t, prog); err == nil {
		t.Error("walk scenario on a standing-only character passed, want a failure")
	}
	if err := scenarioJump(t, prog); err == nil {
		t.Error("jump scenario on a standing-only character passed, want a failure")
	}
}

// scenarioWalkIntoKfm: kfm_zss (P1) holds forward for 30 ticks into the .cns
// kfm (P2) standing 60 ahead. Ikemen GO pushes them apart once their size
// boxes overlap (CharList.pushDetection, char.go:14551-14779): P1's front 16
// and P2's front 16 (kfm.const, kfm.cns [Size] ground.front) leave them 32
// apart, and both weigh 100 with pushfactor 1 (defaults, char.go:416-417),
// so each overlap is split in half. P1 walks 30 * 2.4 = 72; the first 28
// close the gap to 32, the other 44 are overlap, 22 for each: P1 ends at
// -70 + 72 - 22 = -20, P2 at -10 + 22 = 12. Their Clsn2 boxes overlap too
// (kfm.air actions 0 and 20), which Ikemen GO requires (char.go:14641-14645).
func scenarioWalkIntoKfm(t *testing.T, walker, standing FighterProgram) error {
	m := newE2EMatch(t, walker)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 60}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := standing.NewRuntime(p2)
	if err != nil {
		return fmt.Errorf("NewRuntime(P2): %w", err)
	}
	m.programs[match.SideP2] = standing
	m.runtimes[match.SideP2] = rt2
	m.state.Fighters[match.SideP2] = p2
	for i := 0; i < 5; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			return err
		}
	}
	for i := 0; i < 30; i++ {
		if err := m.step(input.TickInput{Right: true}); err != nil {
			return err
		}
		x1, x2 := m.state.Fighters[match.SideP1].Position.X, m.state.Fighters[match.SideP2].Position.X
		if x2-x1 < 32-0.01 {
			return fmt.Errorf("tick %d: P1 at %v, P2 at %v, %v apart, inside the 32 of their size boxes", m.tick, x1, x2, x2-x1)
		}
	}
	x1, x2 := m.state.Fighters[match.SideP1].Position.X, m.state.Fighters[match.SideP2].Position.X
	if !approx(x1, -20) || !approx(x2, 12) {
		return fmt.Errorf("after 30 walking ticks P1, P2 X = %v, %v, want -20, 12 (32 apart, P2 pushed 22)", x1, x2)
	}
	return nil
}

func TestE2E_KfmZssVsKfm_WalkIntoTheOpponent_StopsAtTheSizeBoxes(t *testing.T) {
	if err := scenarioWalkIntoKfm(t, loadReferenceProgram(t, kfmZssDef), loadReferenceProgram(t, kfmCnsDef)); err != nil {
		t.Fatal(err)
	}
}

// Item 060: walking (common1.cns state 20, no ctrl in its header) keeps
// control, so after a walk kfm can still punch: x enters state 200
// (kfm.cmd:630-637), as in Ikemen GO.
func TestE2E_KfmCns_WalkThenPressX_StillPunches(t *testing.T) {
	m := newE2EMatch(t, loadReferenceProgram(t, kfmCnsDef))
	for i := 0; i < 15; i++ {
		if err := m.step(input.TickInput{Right: true}); err != nil {
			t.Fatal(err)
		}
	}
	if s, _, _ := m.p1(); s != 20 {
		t.Fatalf("StateNo = %d after walking, want 20", s)
	}
	for i := 0; i < 3; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.step(input.TickInput{Buttons: map[string]bool{"x": true}}); err != nil {
		t.Fatal(err)
	}
	if s, _, _ := m.p1(); s != 200 {
		t.Errorf("StateNo = %d after pressing x, want 200 (stand light punch)", s)
	}
}
