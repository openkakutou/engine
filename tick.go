package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/combat"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/hitdetect"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/engine/physics"
	"github.com/openkakutou/engine/round"
	"github.com/openkakutou/engine/statemachine"
	"github.com/openkakutou/engine/zssexec"
	"github.com/openkakutou/stage"
)

// FighterProgram bundles one fighter's loaded character data needed to
// drive its own simulation each tick: its state definitions, its animation
// frames, and its command file for input recognition. Read-only and
// unchanged across a whole match -- loaded once, not threaded tick-to-tick
// (see FighterRuntime for what is).
//
// The states come in two formats, as in Ikemen GO where one state table
// holds both: States (.cns) and a compiled .zss script (see WithScripts and
// NewFighterProgramFromFiles). A state number defined in both resolves to
// States -- NewFighterProgramFromFiles drops the .cns definition when a .zss
// file defined it first, so that the first file in load order wins
// (decision 024).
type FighterProgram struct {
	States     map[int]cns.StateDef `json:"states"`
	Animations []air.Animation      `json:"animations"`
	Commands   cmd.CommandFile      `json:"commands"`

	// animIndex is Animations precomputed into a map keyed by Number, built
	// once by NewFighterProgram -- see this item's own ADR (.vibe/decisions/
	// 013). It stays nil on a FighterProgram built via a raw struct literal
	// (this package's own tests, integration_test.go) or JSON-decoded
	// directly (cmd/wasm's incoming request shape, whose field tags above
	// only ever populate Animations, never this unexported field); findAnimation
	// falls back to scanning Animations in that case, so neither of those
	// existing construction paths needs to change.
	animIndex map[int]air.Animation

	// Constants is the character's constants table (Const(name), and the
	// physics of backlog item 043), read from the .def's "cns" file by
	// LoadFighterProgram. Nil reads Ikemen GO's defaults. Not part of the
	// JSON request shape.
	Constants *evaluator.Constants `json:"-"`

	// script is the compiled .zss of the character, set by WithScripts; the
	// zero value defines no state. Unexported like animIndex: a program
	// decoded from JSON has none until the caller runs WithScripts.
	script zssexec.CompiledScript
}

// WithScripts returns p with scripts, the character's .zss files in load
// order (Ikemen GO: st, stN, then stcommon), compiled once so that Tick does
// not parse them. Their Statedefs join p's state table, and a Function is
// shared by every script, the first loaded winning even over the common
// file's (Ikemen GO src/compiler.go:8500-8503, "the first loaded one
// overrides others"). A parse error in a block is reported when that block is
// entered, naming it (zssexec.CompiledScript.Errors lists them all).
func (p FighterProgram) WithScripts(scripts []zss.Script) FighterProgram {
	var all zss.Script
	for _, script := range scripts {
		all.Blocks = append(all.Blocks, script.Blocks...)
	}
	p.script = zssexec.Compile(all)
	return p
}

// StateFile is one state file of a character in load order: either .cns
// states (States) or a .zss script (Script, when not nil).
type StateFile struct {
	States []cns.StateDef
	Script *zss.Script
}

// NewFighterProgramFromFiles builds a FighterProgram from the character's
// state files in Ikemen GO's load order -- every "st" and "stN", the .cmd,
// then "stcommon" (src/compiler.go:8776-8798) -- mixing .cns and .zss. The
// first definition of a state number wins whatever its format, as Ikemen GO
// skips a state already declared (:7538-7539, and in stateCompileZSS for a
// non-negative state: `if _, ok := states[c.stateNo]; !ok || c.stateNo < 0`).
//
// shortcut: Ikemen GO appends a negative state defined in several .zss files
// (:8466-8467); here the first file's negative state wins, as the conformance
// harness already does for .cns. Upgrade when a real character splits -1 or
// -3 across files and the later part matters (kfm_zss's AI.zss -1, the AI
// code, is dropped).
func NewFighterProgramFromFiles(files []StateFile, animations []air.Animation, commands cmd.CommandFile) FighterProgram {
	states := map[int]cns.StateDef{}
	zssNumbers := map[int]bool{}
	var scripts []zss.Script
	for _, f := range files {
		if f.Script != nil {
			scripts = append(scripts, *f.Script)
			for _, b := range f.Script.Blocks {
				if _, inCns := states[b.Number]; b.Kind == zss.BlockKindStatedef && b.NumberRaw == "" && !inCns {
					zssNumbers[b.Number] = true
				}
			}
			continue
		}
		for _, def := range f.States {
			if _, exists := states[def.Number]; exists || zssNumbers[def.Number] {
				continue
			}
			states[def.Number] = def
		}
	}
	return NewFighterProgram(states, animations, commands).WithScripts(scripts)
}

// entryHeader is a Statedef header as enterState applies it.
type entryHeader struct {
	cns.StateDef
	// script reports a .zss header (zssexec.Header): every value field it
	// writes is in HeaderExprs, so an absent ctrl was omitted. A .cns
	// StateDef cannot tell an omitted ctrl from "ctrl = 0".
	script bool
}

// stateHeader returns the header of state number -- a .zss Statedef's
// header converted (zssexec.Header) -- and whether the program defines it in
// either format.
func (p FighterProgram) stateHeader(number int) (entryHeader, bool) {
	if def, ok := p.States[number]; ok {
		return entryHeader{StateDef: def}, true
	}
	def, ok := p.script.Header(number)
	return entryHeader{StateDef: def, script: true}, ok
}

// hasState reports whether number is defined in either format: the check of
// every ChangeState target, as Ikemen GO changes state in one table.
func (p FighterProgram) hasState(number int) bool {
	_, ok := p.stateHeader(number)
	return ok
}

// stateStep is the outcome of running one state of either format.
type stateStep struct {
	Context evaluator.Context
	// HasHitDef reports whether a HitDef controller applied; HitDef is the
	// first one, the same shape for both formats.
	HasHitDef    bool
	HitDef       cns.Controller
	Sounds       []evaluator.SoundEvent
	Unsupported  []string
	SkippedCalls []string // .zss only
}

// step runs the controllers of state number against ctx -- the .cns
// StateDef or the .zss Statedef block, whichever the program defines --
// outside a hit pause, or, when frozen, only what ignorehitpause lets run.
func (p FighterProgram) step(ctx evaluator.Context, number int, frozen bool) (stateStep, error) {
	// The four calls are written out, not picked through a function
	// variable: an indirect call makes p.hasState escape, one allocation per
	// call (decision 009's discipline).
	if _, ok := p.States[number]; ok {
		var r statemachine.Result
		var err error
		if frozen {
			r, err = statemachine.StepStateDuringHitPauseWith(ctx, number, p.States, p.hasState)
		} else {
			r, err = statemachine.StepStateWith(ctx, number, p.States, p.hasState)
		}
		out := stateStep{Context: r.Context, Sounds: r.Sounds, Unsupported: r.Unsupported}
		controllers := p.States[number].Controllers
		for _, i := range r.Applied {
			if strings.EqualFold(controllers[i].Type, statemachine.ControllerTypeHitDef) {
				out.HitDef = controllers[i]
				out.HasHitDef = true
				break
			}
		}
		return out, err
	}
	if !p.hasState(number) {
		return stateStep{}, fmt.Errorf("state %d not found in loaded character", number)
	}
	var r zssexec.Result
	var err error
	if frozen {
		r, err = zssexec.StepStateDuringHitPause(ctx, number, p.script, p.hasState)
	} else {
		r, err = zssexec.StepState(ctx, number, p.script, p.hasState)
	}
	return stateStep{Context: r.Context, HasHitDef: r.HasHitDef, HitDef: r.HitDef, Sounds: r.Sounds, Unsupported: r.Unsupported, SkippedCalls: r.SkippedCalls}, err
}

// NewFighterProgram builds a FighterProgram with its animation lookup
// precomputed once, so tickFighter's per-tick findAnimation call is an O(1)
// map lookup instead of an O(n) scan over animations for the rest of the
// match -- see this item's own ADR. Precomputing once here, rather than
// lazily on first lookup, matters because tickFighter performs exactly one
// lookup per fighter per tick: building the index on that same call would
// cost strictly more (build once *and* look up) than the scan it replaces,
// so the index must be built outside the tick loop, at load time, to pay
// off at all.
//
// When animations contains more than one entry with the same Number, the
// first one (in slice order) is the one findAnimation returns -- matching
// the original linear scan's own first-match behavior on the same
// malformed input, not silently flipped to last-occurrence-wins by a naive
// overwriting map-build loop.
func NewFighterProgram(states map[int]cns.StateDef, animations []air.Animation, commands cmd.CommandFile) FighterProgram {
	index := make(map[int]air.Animation, len(animations))
	for _, a := range animations {
		if _, exists := index[a.Number]; exists {
			continue
		}
		index[a.Number] = a
	}

	return FighterProgram{
		States:     states,
		Animations: animations,
		Commands:   commands,
		animIndex:  index,
	}
}

// FighterRuntime is one fighter's evolving simulation state, threaded
// tick-to-tick by the caller -- the same "caller threads it, this package
// holds no hidden state of its own" convention every other engine package
// with per-tick carried state already follows (evaluator.Context via
// statemachine, input.State via input.Step, combat.ComboState via
// combat.ApplyHits). Build the first one with NewFighterRuntime.
type FighterRuntime struct {
	Context evaluator.Context
	Input   input.State
	Combo   combat.ComboState
	// HitPause is the attacker's remaining frozen ticks after a landed
	// hit (HitDef pausetime[0]). It is not trigger-visible, so it lives
	// here rather than on evaluator.Context; the defender's own freeze is
	// Context.GetHit.HitShakeTime. See .vibe/decisions/018.
	HitPause int
	// Unsupported is the controller types that fired unimplemented in the
	// last tick this runtime went through, in a buffer reused tick after
	// tick so reporting allocates nothing once it has grown. TickResult
	// exposes it: read it before the next Tick, which overwrites it.
	Unsupported []string
	// SkippedCalls is the .zss calls skipped in the last tick (undefined
	// function, wrong argument or return count), in a buffer reused like
	// Unsupported.
	SkippedCalls []string
}

// NewFighterRuntime builds a fresh FighterRuntime for a fighter entering
// states[fighter.StateNo] for the first time -- typically match start or a
// round reset (see round.ResetRound). The fighter enters the state as Tick
// enters any later one (enterState), then gets control as the round starts
// (Ikemen GO system.go:3364, setSCF(SCF_ctrl, true) as the intro ends).
//
// Returns a descriptive error, rather than a runtime built against a
// nonexistent state, if fighter.StateNo is not present in states.
func NewFighterRuntime(fighter match.FighterState, states map[int]cns.StateDef) (FighterRuntime, error) {
	def, ok := states[fighter.StateNo]
	if !ok {
		return FighterRuntime{}, fmt.Errorf("engine: NewFighterRuntime: state %d not found in loaded character", fighter.StateNo)
	}

	ctx := evaluator.Context{FighterState: fighter}
	if _, err := enterState(&ctx, entryHeader{StateDef: def}); err != nil {
		return FighterRuntime{}, fmt.Errorf("engine: NewFighterRuntime: %w", err)
	}
	ctx.Ctrl = true
	return FighterRuntime{Context: ctx}, nil
}

// NewRuntime is NewFighterRuntime for a fighter whose first state may be
// defined in .zss as well as .cns: the fighter enters fighter.StateNo with
// the Statedef header of whichever format defines it. It returns a
// descriptive error if neither does.
func (p FighterProgram) NewRuntime(fighter match.FighterState) (FighterRuntime, error) {
	def, ok := p.stateHeader(fighter.StateNo)
	if !ok {
		return FighterRuntime{}, fmt.Errorf("engine: NewRuntime: state %d not found in loaded character", fighter.StateNo)
	}
	ctx := evaluator.Context{FighterState: fighter, Consts: p.Constants, Anims: p.animationTable()}
	if _, err := enterState(&ctx, def); err != nil {
		return FighterRuntime{}, fmt.Errorf("engine: NewRuntime: %w", err)
	}
	ctx.Ctrl = true // round start, as NewFighterRuntime
	return FighterRuntime{Context: ctx}, nil
}

// animationTable is the animations by number the evaluator reads
// (Context.Anims): the precomputed index, or one built now for a program made
// without NewFighterProgram.
func (p FighterProgram) animationTable() map[int]air.Animation {
	if p.animIndex != nil {
		return p.animIndex
	}
	return NewFighterProgram(nil, p.Animations, p.Commands).animIndex
}

// TickResult is the outcome of one Tick call: the updated MatchState, both
// fighters' updated FighterRuntime (index by match.Side), and whether/how
// the round ended this tick.
type TickResult struct {
	State    match.MatchState
	Fighters [2]FighterRuntime
	Round    round.RoundResult
	// Sounds is each fighter's PlaySnd events recorded this tick, indexed
	// by match.Side like every other per-side result here -- nil for a
	// side that triggered none, matching this package's idiomatic Go API
	// (JSON-null-vs-empty-array normalization is cmd/wasm's own
	// boundary-only responsibility). See .vibe/decisions/017.
	Sounds [2][]evaluator.SoundEvent
	// Unsupported is each fighter's controller types that fired this tick
	// without an implementation, lower-cased and deduplicated across the
	// special states and the current state -- nil for a side with none.
	// It shares storage with the runtime's reused buffer: read it before the
	// next Tick, which overwrites it. See .vibe/decisions/020.
	Unsupported [2][]string
	// SkippedCalls is each fighter's .zss calls that did nothing this tick
	// (an undefined function, a wrong argument or return count), as the
	// Ikemen GO console logs them (bytecode.go:5050-5069) -- nil for a side
	// with none. Same reused storage as Unsupported: read it before the next
	// Tick.
	SkippedCalls [2][]string
}

// TickConfig bundles Tick's simulation-constant parameters (bounds,
// gravity, comboWindow) together with the current simulation tick number,
// which the caller advances by exactly 1 between successive calls (see
// input.Step). bounds is the current stage's horizontal boundaries;
// gravity and comboWindow are simulation constants the caller supplies
// (engine holds no default balance data of its own, matching physics.Step
// and combat.ApplyHits's own existing contracts).
//
// Passed to Tick by value, not by pointer: at this size (a pointer plus
// three scalars) it is cheap to copy and Go's calling convention passes it
// in registers, so a caller building a fresh TickConfig{...} literal at
// each call site -- as cmd/wasm's tickJS does -- allocates nothing. Taking
// its address instead would be a classic escape-analysis trap, reintroducing
// a per-tick heap allocation this repo's allocation-discipline ADRs
// (.vibe/decisions/009-011) specifically exist to avoid.
type TickConfig struct {
	Bounds      *stage.StageBoundaries
	Gravity     float64
	Tick        int
	ComboWindow int
}

// Tick advances a full simulation tick for both fighters -- the one
// function tying every prior engine package into a usable match loop: for
// each fighter, input recognition, state execution (.cns or .zss, whichever
// format defines the state), and
// current-animation-frame resolution; then, once for the pair, hit
// detection; then, per fighter whose state's HitDef triggered this tick,
// damage/combo resolution; then, per fighter, physics; then a round-outcome
// check. This is the function the WASM entrypoint (cmd/wasm) exposes for a
// `game` app to drive a match one tick at a time -- see this item's
// own ADR for the scope this function deliberately does and does not cover.
//
// programs is read-only character data, loaded once per fighter outside
// the simulation loop. runtimes is each fighter's evolving state as of the
// start of this tick (see FighterRuntime); inputs is this tick's raw
// device input per fighter. cfg carries this tick's simulation constants
// and current tick number -- see TickConfig.
//
// Returns a descriptive error, never a panic, if either fighter's current
// state is not present in its own loaded FighterProgram, or if physics
// rejects the given stage boundaries.
func Tick(
	state match.MatchState,
	programs [2]FighterProgram,
	runtimes [2]FighterRuntime,
	inputs [2]input.TickInput,
	cfg TickConfig,
) (TickResult, error) {
	// Ikemen GO runs the fighters one after the other (char.go:14066-14090),
	// each acting then moving (actionRun, char.go:12583): the first sees the
	// second as the previous tick left it, the second sees the first after
	// its controllers and its movement; collision detection comes after both
	// (system.go:2734). See decisions 027 and 029.
	p2Seen := snapshotOf(state.Fighter(match.SideP2), &runtimes[match.SideP2].Context)
	p1Out, err := tickFighter(programs[match.SideP1], runtimes[match.SideP1], state.Fighter(match.SideP1), p2Seen, inputs[match.SideP1], cfg.Tick, cfg.Bounds, cfg.Gravity, state.Round, state.RandSeed)
	if err != nil {
		return TickResult{}, fmt.Errorf("engine: Tick: side %v: %w", match.SideP1, err)
	}
	// Both fighters draw Random from one sequence, P1 first, as Ikemen GO's
	// single seed is advanced in player order; the seed rides in match state.
	state.RandSeed = p1Out.Runtime.Context.RandSeed
	// What the controllers did to position, velocity, facing, health and
	// power is the fighter's new match state: hit detection below and the
	// next fighter's tick see it, as in Ikemen GO where a controller's
	// effect is immediate.
	state.Fighters[match.SideP1] = p1Out.Runtime.Context.FighterState
	p1Seen := snapshotOf(state.Fighter(match.SideP1), &p1Out.Runtime.Context)
	p2Out, err := tickFighter(programs[match.SideP2], runtimes[match.SideP2], state.Fighter(match.SideP2), p1Seen, inputs[match.SideP2], cfg.Tick, cfg.Bounds, cfg.Gravity, state.Round, state.RandSeed)
	if err != nil {
		return TickResult{}, fmt.Errorf("engine: Tick: side %v: %w", match.SideP2, err)
	}

	state.RandSeed = p2Out.Runtime.Context.RandSeed
	state.Fighters[match.SideP2] = p2Out.Runtime.Context.FighterState

	var frames [2]air.Frame
	frames[match.SideP1] = p1Out.Frame
	frames[match.SideP2] = p2Out.Frame
	events := hitdetect.Detect(&state, frames)

	var runtimesOut [2]FighterRuntime
	runtimesOut[match.SideP1] = p1Out.Runtime
	runtimesOut[match.SideP2] = p2Out.Runtime

	if p1Out.HasHitDef {
		damage := combat.DamageParams{Damage: p1Out.HitDef.Parameters["damage"]}
		newState, newCombo, hit := combat.ApplyHits(state, events, match.SideP1, damage, runtimesOut[match.SideP1].Combo, cfg.Tick, cfg.ComboWindow)
		state = newState
		runtimesOut[match.SideP1].Combo = newCombo
		if err := applyHitReaction(&programs, &runtimesOut, match.SideP1, p1Out.HitDef.Parameters, hit); err != nil {
			return TickResult{}, fmt.Errorf("engine: Tick: side %v: %w", match.SideP1, err)
		}
	}
	if p2Out.HasHitDef {
		damage := combat.DamageParams{Damage: p2Out.HitDef.Parameters["damage"]}
		newState, newCombo, hit := combat.ApplyHits(state, events, match.SideP2, damage, runtimesOut[match.SideP2].Combo, cfg.Tick, cfg.ComboWindow)
		state = newState
		runtimesOut[match.SideP2].Combo = newCombo
		if err := applyHitReaction(&programs, &runtimesOut, match.SideP2, p2Out.HitDef.Parameters, hit); err != nil {
			return TickResult{}, fmt.Errorf("engine: Tick: side %v: %w", match.SideP2, err)
		}
	}

	// A get-hit state entered by applyHitReaction and a PowerAdd live on the
	// runtime context: copy them onto the match state.
	state.Fighters[match.SideP1].StateNo = runtimesOut[match.SideP1].Context.StateNo
	state.Fighters[match.SideP1].Power = runtimesOut[match.SideP1].Context.Power
	state.Fighters[match.SideP2].StateNo = runtimesOut[match.SideP2].Context.StateNo
	state.Fighters[match.SideP2].Power = runtimesOut[match.SideP2].Context.Power

	var sounds [2][]evaluator.SoundEvent
	sounds[match.SideP1] = p1Out.Sounds
	sounds[match.SideP2] = p2Out.Sounds

	var unsupported [2][]string
	unsupported[match.SideP1] = nilIfEmpty(p1Out.Runtime.Unsupported)
	unsupported[match.SideP2] = nilIfEmpty(p2Out.Runtime.Unsupported)

	var skippedCalls [2][]string
	skippedCalls[match.SideP1] = nilIfEmpty(p1Out.Runtime.SkippedCalls)
	skippedCalls[match.SideP2] = nilIfEmpty(p2Out.Runtime.SkippedCalls)

	// A RoundTimer of 0 at the start of a tick is an untimed round
	// (MUGEN/Ikemen's "unlimited time" convention): it neither counts down
	// nor ends on time. Otherwise the timer counts down one tick, and
	// reaching 0 is a timeout, which CheckOutcome resolves.
	untimed := state.RoundTimer <= 0
	if !untimed {
		state.RoundTimer--
	}

	result := round.CheckOutcome(&state)
	if untimed && (result.Outcome == round.OutcomeTimeout || result.Outcome == round.OutcomeTimeoutDraw) {
		result = round.RoundResult{}
	}

	return TickResult{
		State:        state,
		Fighters:     runtimesOut,
		Round:        result,
		Sounds:       sounds,
		Unsupported:  unsupported,
		SkippedCalls: skippedCalls,
	}, nil
}

// fighterTickOutput is tickFighter's result -- kept as a plain returned
// struct (not a shared mutable field) so the two calls Tick makes (one per
// side, explicitly unrolled rather than looped, per this item's own
// allocation-discipline ADR) stay independent.
type fighterTickOutput struct {
	Runtime   FighterRuntime
	Frame     air.Frame
	HasHitDef bool
	HitDef    cns.Controller
	Sounds    []evaluator.SoundEvent
	// Frozen reports the fighter was in hit pause (attacker freeze or
	// defender shake) this tick: only controllers flagged ignorehitpause
	// ran, Time/AnimElapsed were held, and Tick skips its physics.
	Frozen bool
}

// specialStates are the negative states run every tick before the current
// one, in Ikemen GO's order (char.go actionRun).
var specialStates = [...]int{-3, -2, -1}

// tickFighter advances one fighter's own per-tick simulation -- input
// recognition, CNS state-machine execution, and current-frame resolution
// -- given fighter, that fighter's live match.FighterState as of the start
// of this tick (position/facing/health, sourced from match.MatchState, the
// single source of truth Tick reconciles runtime.Context against every
// call).
func tickFighter(prog FighterProgram, runtime FighterRuntime, fighter match.FighterState, opponent evaluator.Opponent, in input.TickInput, tick int, bounds *stage.StageBoundaries, gravity float64, round int, seed int32) (fighterTickOutput, error) {
	ctx := runtime.Context
	ctx.FighterState = fighter
	ctx.P2 = opponent
	// A target that left its hit states is dropped before the fighter acts
	// (Ikemen GO char.go:12490, 10387-10404).
	if opponent.MoveType != 'H' {
		ctx.HasTarget = false
	}
	ctx.Round = round
	ctx.RandSeed = seed
	ctx.YAccel = gravity

	newInputState, active := input.Step(runtime.Input, tick, fighter.Facing, in, prog.Commands)
	ctx.ActiveCommands = active

	// The frame resolved here is the one active as this tick begins --
	// i.e. before this tick's own state-machine transition (if any) is
	// applied. A transition's new animation takes effect starting next
	// tick, once AnimElapsed has actually reset -- see this item's own ADR.
	frame := currentFrame(findAnimation(prog, ctx.Anim), ctx.AnimElapsed)

	if !prog.hasState(ctx.StateNo) {
		return fighterTickOutput{}, fmt.Errorf("current state %d not found in loaded character", ctx.StateNo)
	}

	// A fighter in hit pause (attacker freeze or defender shake) runs only
	// the controllers flagged ignorehitpause, advances no Time/AnimElapsed and
	// moves no physics; input recognition above keeps running. See
	// .vibe/decisions/018 and 021.
	frozen := runtime.HitPause > 0 || ctx.GetHit.HitShakeTime > 0

	// Ikemen GO's actionRun (char.go): states -3, -2 then -1 run every
	// tick, each as its own phase, before the current state. A ChangeState
	// stops only the state it is in; the next phase still runs and sees the
	// new StateNo. A state change made by them is entered (stateChange2)
	// before the current state -- now the new one -- runs, in this tick.
	var sounds []evaluator.SoundEvent
	unsupported := runtime.Unsupported[:0]
	skippedCalls := runtime.SkippedCalls[:0]
	ctx.StateChanged = false
	basicActions(&ctx, prog, in.Directions(fighter.Facing))
	for _, number := range specialStates {
		if !prog.hasState(number) {
			continue
		}
		special, err := prog.step(ctx, number, frozen)
		if err != nil {
			return fighterTickOutput{}, err
		}
		ctx = special.Context
		sounds = append(sounds, special.Sounds...)
		unsupported = appendMissing(unsupported, special.Unsupported)
		skippedCalls = appendMissing(skippedCalls, special.SkippedCalls)
	}
	// A ChangeState made by -3/-2/-1 -- even into the current number -- or
	// one left pending by a hit pause is entered now, before the current
	// state runs (Ikemen GO stateChange2, char.go:12529). During a hit pause
	// the entry waits (char.go:6943-6944).
	enter := ctx.StateChanged || (ctx.StateEntryPending && !frozen)
	if enter && frozen {
		ctx.StateEntryPending = true
	}
	ctx.StateChanged = false
	if enter && !frozen {
		ctx.StateEntryPending = false
		entered, _ := prog.stateHeader(ctx.StateNo)
		if _, err := enterState(&ctx, entered); err != nil {
			return fighterTickOutput{}, err
		}
		// Ikemen GO applies the new animation in stateChange2, before
		// collision detection: this tick uses its first frame (char.go:6943,
		// 13049-13062).
		frame = currentFrame(findAnimation(prog, ctx.Anim), ctx.AnimElapsed)
	}

	result, err := prog.step(ctx, ctx.StateNo, frozen)
	if err != nil {
		return fighterTickOutput{}, err
	}
	sounds = append(sounds, result.Sounds...)
	unsupported = appendMissing(unsupported, result.Unsupported)
	skippedCalls = appendMissing(skippedCalls, result.SkippedCalls)

	newCtx := result.Context
	resolveAnimElem(&newCtx, findAnimation(prog, newCtx.Anim))
	changed := newCtx.StateChanged
	newCtx.StateChanged = false
	if frozen {
		if changed {
			newCtx.StateEntryPending = true
		}
		frozenRuntime := FighterRuntime{Context: newCtx, Input: newInputState, Combo: runtime.Combo, HitPause: runtime.HitPause, Unsupported: unsupported, SkippedCalls: skippedCalls}
		if frozenRuntime.HitPause > 0 {
			frozenRuntime.HitPause--
		}
		if frozenRuntime.Context.GetHit.HitShakeTime > 0 {
			frozenRuntime.Context.GetHit.HitShakeTime--
		}
		if runtime.HitPause == 0 { // the defender's shake steps the fall time; an attacker's pause does not
			stepFall(&frozenRuntime.Context)
		}
		return fighterTickOutput{Runtime: frozenRuntime, Frame: frame, Frozen: true, Sounds: sounds}, nil
	}
	if changed {
		// statemachine's own ChangeState handling already reset Time to 0
		// (character/cmd data plays no part in that); Anim/AnimElapsed/Ctrl
		// are enterState's responsibility, since statemachine has no
		// character-loadout knowledge of the target state's declared
		// values.
		entered, _ := prog.stateHeader(newCtx.StateNo)
		restarted, err := enterState(&newCtx, entered)
		if err != nil {
			return fighterTickOutput{}, err
		}
		if !restarted {
			newCtx.AnimElapsed++ // a header without anim keeps the animation running
		}
	} else {
		newCtx.Time++
		newCtx.AnimElapsed++
	}
	// Ikemen GO actionRun moves the fighter after its states (posUpdate,
	// char.go:12583), then lands it (char.go:12584-12590).
	moved, err := physics.Step(newCtx.FighterState, bounds, gravityFor(&newCtx, gravity))
	if err != nil {
		return fighterTickOutput{}, fmt.Errorf("physics: %w", err)
	}
	newCtx.FighterState = moved
	if err := land(&newCtx, prog); err != nil {
		return fighterTickOutput{}, err
	}
	// After every unfrozen action, a state change included, the contact
	// counter grows (char.go:12594-12596); a state entered without
	// movehitpersist has just reset it to 0.
	if newCtx.MoveContactTime > 0 {
		newCtx.MoveContactTime++
	}
	stepFall(&newCtx)
	// Once the shake is over the get-hit time counts down on every
	// unfrozen tick, stopping at -1 (HitOver).
	if newCtx.GetHit.Active && newCtx.GetHit.HitTime >= 0 {
		newCtx.GetHit.HitTime--
	}

	// The HitDef that just applied becomes the one HitDefAttr reads, unless
	// the state changed in the same tick: entering the new state has just
	// reset it (enterState, hitdefpersist).
	if result.HasHitDef && !changed {
		if attr, err := evaluator.ParseAttr(result.HitDef.Parameters["attr"], true); err == nil {
			newCtx.HitDefAttr = attr
		}
	}

	return fighterTickOutput{
		Runtime:   FighterRuntime{Context: newCtx, Input: newInputState, Combo: runtime.Combo, Unsupported: unsupported, SkippedCalls: skippedCalls},
		Frame:     frame,
		HasHitDef: result.HasHitDef,
		HitDef:    result.HitDef,
		Sounds:    sounds,
	}, nil
}

// land is Ikemen GO's hard-coded landing (char.go:12584-12590): a fighter in
// physics A falling (velocity down) at or below the ground, outside state
// 105, changes to state 52, entered like any ChangeState (enterState).
// Ikemen GO runs state 52 at once (changeStateEx, char.go:6987-6994), whose
// VelSet y = 0 and PosSet y = 0 at Time = 0 put it on the ground on this tick
// (common1.cns:273-281); the engine runs a newly entered state's controllers
// on the next tick, so it puts the fighter on the ground here, horizontal
// velocity kept, for the landing tick's position to be Ikemen GO's.
// shortcut: a character without state 52 is left falling, as basicChangeState
// does, where Ikemen GO enters an empty state.
func land(ctx *evaluator.Context, prog FighterProgram) error {
	if ctx.Physics != 'A' || ctx.Velocity.Y >= 0 || ctx.Position.Y > 0 || ctx.StateNo == 105 {
		return nil
	}
	def, ok := prog.stateHeader(52)
	if !ok {
		return nil
	}
	ctx.PrevStateNo = ctx.StateNo
	ctx.StateNo = 52
	ctx.Position.Y = 0
	ctx.Velocity.Y = 0
	_, err := enterState(ctx, def)
	return err
}

// basicActions are the walk, jump and crouch a fighter with control gets
// from the engine itself, before its states -3, -2 and -1 run (Ikemen GO
// actionPrepare, char.go:12300-12345; common1 has no ChangeState into 20 or
// 40). Like a ChangeState there, the change is entered after state -1.
// shortcut: no AssertSpecial yet (item 050), so the nohardcodedkeys,
// nojump, nocrouch, nostand, nowalk and nobrake flags are all off, as in
// Ikemen GO without AssertSpecial; there is no guard distance, so the guard
// entry (120) never fires and back does not cancel forward (char.go:12306,
// 12332); the air jump (45) and the round-over input lock (char.go:12309)
// are not done.
func basicActions(ctx *evaluator.Context, prog FighterProgram, d input.Directions) {
	if ctx.Ctrl {
		switch {
		case ctx.StateType == 'S' && d.Up:
			if ctx.StateNo != 40 {
				basicChangeState(ctx, prog, 40)
			}
		case ctx.StateType == 'S' && d.Down:
			if ctx.StateNo != 10 {
				if ctx.StateNo != 100 {
					ctx.Velocity.X = 0
				}
				basicChangeState(ctx, prog, 10)
			}
		case ctx.StateType == 'C' && !d.Down:
			if ctx.StateNo != 12 {
				basicChangeState(ctx, prog, 12)
			}
		case ctx.StateType == 'S' && d.Forward != d.Back:
			if ctx.StateNo != 20 {
				basicChangeState(ctx, prog, 20)
			}
		}
	}
	// Braking does not need control (char.go:12338-12342).
	if ctx.StateNo == 20 && d.Back == d.Forward {
		basicChangeState(ctx, prog, 0)
	}
}

// basicChangeState changes to state number as a ChangeState with no anim or
// ctrl does (statemachine applyChangeState).
// shortcut: a character without that state is left where it is, where
// Ikemen GO enters an empty state (char.go:6852-6857); every character
// loading common1 has them.
func basicChangeState(ctx *evaluator.Context, prog FighterProgram, number int) {
	if !prog.hasState(number) {
		return
	}
	ctx.PrevStateNo = ctx.StateNo
	ctx.StateNo = number
	ctx.Time = 0
	ctx.StateChanged = true
	ctx.Persistent = [evaluator.MaxPersistent]int32{}
}

// nilIfEmpty returns names, or nil when it is empty, so a side with nothing
// to report reads as nil though its buffer is kept.
func nilIfEmpty(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	return names
}

// appendMissing appends to dst each of src's names it does not hold yet.
func appendMissing(dst, src []string) []string {
	for _, name := range src {
		if !slices.Contains(dst, name) {
			dst = append(dst, name)
		}
	}
	return dst
}

// resolveAnimElem converts ctx's pending ChangeAnim element request into an
// AnimElapsed within anim and clears the request. Ikemen GO char.go
// setAnimElem: an element past the animation's last becomes 1 with time 0,
// and an element time that is negative or not shorter than the element's own
// duration becomes 0. An animation without frames (unknown to the character)
// leaves AnimElapsed at the 0 ChangeAnim set. A frame with no positive duration
// (held indefinitely) adds nothing to the start of a later element.
func resolveAnimElem(ctx *evaluator.Context, anim air.Animation) {
	elem, elemTime := ctx.AnimElem, ctx.AnimElemTime
	ctx.AnimElem, ctx.AnimElemTime = 0, 0
	if elem == 0 || len(anim.Frames) == 0 {
		return
	}
	if elem > len(anim.Frames) {
		elem, elemTime = 1, 0
	}
	if frame := anim.Frames[elem-1]; elemTime < 0 || (frame.Time > 0 && elemTime >= frame.Time) {
		elemTime = 0
	}
	start := 0
	for _, frame := range anim.Frames[:elem-1] {
		start += max(frame.Time, 0)
	}
	ctx.AnimElapsed = start + elemTime
}

// enterState applies the bookkeeping of entering the state def to ctx,
// whose StateNo is already the new state's, in Ikemen GO's order:
// stateChange1 restarts Time and the persistent counters, StateBytecode.init
// sets the types (bytecode.go:17032-17076), then stateDef.Run runs the
// header fields that are written, each evaluated now (bytecode.go:5444-5512;
// compiler.go:7020-7130 makes all but the persist flags optional). It reports
// whether the header restarted the animation: without anim, the current one
// keeps running. Shared by tickFighter's state changes, NewRuntime and the
// get-hit entry. Juggle points, sprpriority and hitcountpersist are not
// modelled and are not read.
func enterState(ctx *evaluator.Context, def entryHeader) (animRestarted bool, err error) {
	ctx.Time = 0
	ctx.Persistent = [evaluator.MaxPersistent]int32{} // Ikemen GO char.go:6896-6903
	// An omitted physics is N (Ikemen GO newStateBytecode); U or an unknown
	// letter keeps the previous one (StateBytecode.init).
	if l := physicsLetter(def.Physics); strings.IndexByte("SCAN", l) >= 0 {
		ctx.Physics = l
	}
	// A state type or move type of U, or one that is not a known letter,
	// leaves the previous one in place (Ikemen GO bytecode.go
	// StateBytecode.init); an omitted one is S and I (newStateBytecode).
	if l := stanceLetter(def.Type); strings.IndexByte("SCAL", l) >= 0 {
		ctx.StateType = l
	}
	if l := moveLetter(def.MoveType); strings.IndexByte("IAH", l) >= 0 {
		ctx.MoveType = l
	}

	// Entering a state clears the contact counter unless movehitpersist,
	// and its HitDef unless hitdefpersist (bytecode.go:5456-5463): attr 0
	// never matches HitDefAttr (char.go:5829-5830).
	persist, err := headerBool(ctx, def, "movehitpersist", def.MoveHitPersist)
	if err != nil {
		return false, err
	}
	if !persist {
		ctx.MoveContactTime = 0
	}
	if persist, err = headerBool(ctx, def, "hitdefpersist", def.HitDefPersist); err != nil {
		return false, err
	}
	if !persist {
		ctx.HitDefAttr = 0
	}

	// facep2 turns toward an opponent behind (bytecode.go:5487-5490,
	// shouldFaceP2 char.go:6775-6800); P2Dist X reads 0 without one.
	faceP2, err := headerBool(ctx, def, "facep2", def.FaceP2)
	if err != nil {
		return false, err
	}
	if faceP2 {
		behind, err := evaluator.EvaluateMut("p2dist x < 0", ctx)
		if err != nil {
			return false, err
		}
		if behind.Bool() && ctx.Facing == match.FacingRight {
			ctx.Facing = match.FacingLeft
		} else if behind.Bool() {
			ctx.Facing = match.FacingRight
		}
	}

	// velset: facing-relative X, Y down in Ikemen GO (bytecode.go:5493-5500).
	// Only a .zss header carries it (zssexec.Header): character's .cns
	// StateDef drops the field.
	x, hasX, err := headerExpr(ctx, def, "velset.x")
	if err != nil {
		return false, err
	}
	if hasX {
		ctx.Velocity.X = 0 + x.Float()
	}
	if hasX && ctx.Facing == match.FacingLeft {
		ctx.Velocity.X = 0 - x.Float()
	}
	y, hasY, err := headerExpr(ctx, def, "velset.y")
	if err != nil {
		return false, err
	}
	if hasY {
		ctx.Velocity.Y = 0 - y.Float()
	}

	// anim: -1 means no change, an omitted one keeps the current animation
	// (bytecode.go:5501-5507). cns.StateDef.Anim 0 stands for omitted.
	anim, hasAnim, err := headerExpr(ctx, def, "anim")
	if err != nil {
		return false, err
	}
	animNo := anim.Int()
	if !hasAnim && def.Anim != 0 {
		animNo, hasAnim = def.Anim, true
	}
	if hasAnim && animNo != -1 {
		ctx.Anim = animNo
		ctx.AnimElapsed = 0
		ctx.AnimElem, ctx.AnimElemTime = 0, 0
		animRestarted = true
	}

	// ctrl (bytecode.go:5508-5510): an omitted one leaves Ctrl unchanged.
	ctrl, hasCtrl, err := headerExpr(ctx, def, "ctrl")
	if err != nil {
		return false, err
	}
	if hasCtrl {
		ctx.Ctrl = ctrl.Bool()
	}
	if !hasCtrl && !def.script {
		// shortcut: a .cns StateDef cannot tell an omitted ctrl from
		// "ctrl = 0", so its typed value applies; upgrade when character
		// records which header fields are written.
		ctx.Ctrl = def.Ctrl
	}

	// poweradd (bytecode.go:5511-5512), clamped as the PowerAdd controller.
	power, hasPower, err := headerExpr(ctx, def, "poweradd")
	if err != nil {
		return false, err
	}
	add := def.PowerAdd
	if hasPower {
		add = power.Int()
	}
	ctx.Power = min(max(ctx.Power+add, 0), statemachine.DefaultMaxPower)
	return animRestarted, nil
}

// headerExpr evaluates the header field name when it is written as an
// expression (cns.StateDef.HeaderExprs; every written value field of a .zss
// header), reporting ok false when it is not.
func headerExpr(ctx *evaluator.Context, def entryHeader, name string) (v evaluator.Value, ok bool, err error) {
	raw, ok := def.HeaderExprs[name]
	if !ok {
		return evaluator.Value{}, false, nil
	}
	v, err = evaluator.EvaluateMut(raw, ctx)
	if err != nil {
		return evaluator.Value{}, false, fmt.Errorf("state %d header %s %q: %w", def.Number, name, raw, err)
	}
	return v, true, nil
}

// headerBool is a boolean header field: its expression when written as one,
// else the literal value.
func headerBool(ctx *evaluator.Context, def entryHeader, name string, literal bool) (bool, error) {
	v, ok, err := headerExpr(ctx, def, name)
	if err != nil || !ok {
		return literal, err
	}
	return v.Bool(), nil
}

// snapshotOf is what the other fighter's triggers read of fighter: its match
// state as of now and the rest from its runtime context.
func snapshotOf(fighter match.FighterState, ctx *evaluator.Context) evaluator.Opponent {
	return evaluator.Opponent{
		Valid:           true,
		FighterState:    fighter,
		Time:            ctx.Time,
		Anim:            ctx.Anim,
		PrevStateNo:     ctx.PrevStateNo,
		Ctrl:            ctx.Ctrl,
		StateType:       ctx.StateType,
		MoveType:        ctx.MoveType,
		GetHit:          ctx.GetHit,
		MoveContactTime: ctx.MoveContactTime,
		HitDefAttr:      ctx.HitDefAttr,
		Consts:          ctx.Consts,
	}
}

// stepFall is what a fighter's tick does to its fall: in a hit state the fall
// time counts while the fall flag is set, and the first tick out of one clears
// the flag (Ikemen GO char.go:12644-12673). canRecover reads the fall time.
func stepFall(ctx *evaluator.Context) {
	switch {
	case ctx.MoveType != 'H':
		ctx.GetHit.Fall = false
	case ctx.GetHit.Fall:
		ctx.GetHit.FallTime++
	}
}

// applyHitReaction applies a landed hit's HitDef to both fighters on the
// tick it lands (a no-op for a miss): the attacker is frozen for
// pausetime[0] ticks; the defender records its get-hit variables, is
// frozen for pausetime[1] ticks, and enters the get-hit state matching its
// current StateDef type (first letter, case-insensitive; omitted = S) --
// S 5000, C 5010, A 5020 -- in the same tick.
// A defender whose stance is L/U, or whose character has no such state,
// keeps its current state (the freeze and variables still apply), no
// error. It returns the error of a get-hit header expression the defender's
// state entry could not evaluate. See .vibe/decisions/018.
func applyHitReaction(programs *[2]FighterProgram, runtimes *[2]FighterRuntime, attacker match.Side, hitDefParams map[string]string, hit combat.HitResult) error {
	if !hit.Hit {
		return nil
	}
	params := combat.ParseHitDef(hitDefParams)
	runtimes[attacker].HitPause = params.PauseTime[0]
	// The contact counter starts at 1 on the tick of the hit (Ikemen GO
	// char.go:14270-14271, 13125-13126), and the defender becomes a target.
	runtimes[attacker].Context.MoveContactTime = 1
	runtimes[attacker].Context.HasTarget = true

	ctx := &runtimes[hit.Defender].Context
	defender := programs[hit.Defender]

	hitTime := params.GroundHitTime
	target := 0
	switch ctx.StateType {
	case 'S':
		target = 5000
	case 'C':
		target = 5010
	case 'A':
		target = 5020
		hitTime = params.AirHitTime
	}
	ctx.GetHit = getHitFor(params, ctx, hitTime)
	// Ikemen GO makes the defender's move type H before entering the get-hit
	// state, whose StateDef may then set its own (char.go:13123-13129).
	ctx.MoveType = 'H'
	if def, ok := defender.stateHeader(target); ok && target != 0 {
		ctx.PrevStateNo = ctx.StateNo
		ctx.StateNo = target
		if _, err := enterState(ctx, def); err != nil {
			return err
		}
	}
	return nil
}

// getHitFor is the defender's get-hit variables for a landed hit, as Ikemen GO
// copies them from the HitDef (char.go:11680-11838): the velocity, fall flag
// and control time of the air branch for a fighter in the air and of the ground
// one otherwise, the fall flag kept from an earlier hit it has not recovered
// from, and the animation type chosen by gethitAnimtype (char.go:8102-8114).
//
// shortcut: a defender lying down (type L) takes the ground branch, where
// Ikemen GO uses down.velocity and down.hittime (char.go:11773-11787); Tick does
// not put a lying fighter into a get-hit state yet.
func getHitFor(params combat.HitDefParams, defender *evaluator.Context, hitTime int) evaluator.GetHit {
	g := evaluator.GetHit{
		Active:          true,
		HitShakeTime:    params.PauseTime[1],
		HitTime:         hitTime,
		SlideTime:       params.GroundSlideTime,
		GroundType:      params.GroundType,
		AirType:         params.AirType,
		Yaccel:          params.YAccel,
		FallYvel:        params.FallYVelocity,
		FallRecover:     params.FallRecover,
		FallRecoverTime: params.FallRecoverTime,
		HitID:           params.HitID,
		Fall:            defender.GetHit.Fall,
	}
	airborne := defender.StateType == 'A'
	if airborne {
		g.Yvel = params.AirVelocityY
		g.Fall = g.Fall || params.AirFall
		g.CtrlTime = params.AirHitTime
	} else {
		g.Yvel = params.GroundVelocityY
		g.Fall = g.Fall || params.GroundFall
		g.CtrlTime = params.GroundHitTime
		if g.Fall && g.Yvel == 0 {
			g.Yvel = -0.001 // char.go:11796-11799, "some form of internal workaround"
		}
	}
	const animTypeBack = 3
	switch {
	case g.Fall:
		g.AnimType = params.FallAnimType
	case airborne:
		g.AnimType = params.AirAnimType
	case params.AnimType >= animTypeBack && g.Yvel == 0:
		g.AnimType = 2 // Hard
	default:
		g.AnimType = params.AnimType
	}
	return g
}

// moveLetter is a StateDef's move type as an upper-case letter: its first
// character, case-insensitive, and 'I' when the line was omitted.
func moveLetter(t cns.MoveType) byte {
	if t == "" {
		return 'I'
	}
	c := t[0]
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	return c
}

// physicsLetter is a StateDef's physics type as an upper-case letter: its
// first character, case-insensitive, and 'N' when the line was omitted.
func physicsLetter(t cns.PhysicsType) byte {
	if t == "" {
		return 'N'
	}
	c := t[0]
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	return c
}

// gravityFor is the gravity physics.Step should apply to a fighter: the
// configured one in physics type A, none otherwise. Ikemen GO's char.go
// posUpdate adds yaccel automatically only for ss.physics == ST_A; any other
// state gets gravity from its Gravity controller.
func gravityFor(ctx *evaluator.Context, gravity float64) float64 {
	if ctx.Physics != 'A' {
		return 0
	}
	return gravity
}

// stanceLetter is a StateDef's type as an upper-case letter: its first
// character, case-insensitive, and 'S' when the type line was omitted (the
// MUGEN 1.1 cns doc: "If this line is omitted, it assumes the type is S").
func stanceLetter(t cns.StateType) byte {
	if t == "" {
		return 'S'
	}
	c := t[0]
	if c >= 'a' && c <= 'z' {
		c -= 'a' - 'A'
	}
	return c
}

// findAnimation returns the Animation in prog whose Number matches number,
// or the zero Animation (no frames) if none matches -- a fighter whose
// current Anim references data its own FighterProgram doesn't carry simply
// contributes no Clsn boxes for that tick, the same "no error path, only
// geometry over whatever's there" stance hitdetect.Detect itself already
// takes on a frame with no boxes at all.
//
// Uses prog's precomputed animIndex (see NewFighterProgram) when present;
// falls back to a linear scan over prog.Animations for a FighterProgram
// built via a raw struct literal or decoded straight from JSON, neither of
// which populates animIndex.
func findAnimation(prog FighterProgram, number int) air.Animation {
	if prog.animIndex != nil {
		return prog.animIndex[number]
	}
	for _, a := range prog.Animations {
		if a.Number == number {
			return a
		}
	}
	return air.Animation{}
}

// currentFrame returns the Frame of anim active at animTime ticks since
// anim started playing, honoring anim's LoopStart once its sequence has
// played through once. A frame whose Time is zero or negative holds
// indefinitely -- MUGEN/Ikemen's own "-1 = infinite" convention for a
// frame's duration, generalized here to "non-positive" so a malformed
// zero-duration frame can never make this function loop forever. Returns
// the zero Frame if anim has no frames at all.
func currentFrame(anim air.Animation, animTime int) air.Frame {
	if len(anim.Frames) == 0 {
		return air.Frame{}
	}
	elem, _ := evaluator.ElementAt(anim, animTime)
	return anim.Frames[elem]
}
