package engine

import (
	"fmt"
	"strings"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/combat"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/hitdetect"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/engine/physics"
	"github.com/openkakutou/engine/round"
	"github.com/openkakutou/engine/statemachine"
	"github.com/openkakutou/stage"
)

// FighterProgram bundles one fighter's loaded character data needed to
// drive its own simulation each tick: its state definitions (CNS-driven
// state-machine execution only -- see this item's own ADR for why .zss
// execution is out of this Tick's scope), its animation frames, and its
// command file for input recognition. Read-only and unchanged across a
// whole match -- loaded once, not threaded tick-to-tick (see
// FighterRuntime for what is).
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
}

// NewFighterRuntime builds a fresh FighterRuntime for a fighter entering
// states[fighter.StateNo] for the first time -- typically match start or a
// round reset (see round.ResetRound). Time and AnimTime start at 0, and
// Anim/Ctrl take the entered state's own declared values (Anim defaulting
// to the state number itself, matching cns.StateDef.Anim's own "0 means
// not set" convention) -- mirroring the bookkeeping Tick itself applies on
// every later state transition, so a fighter's very first state is not a
// special case for any later trigger relying on Anim/Ctrl.
//
// Returns a descriptive error, rather than a runtime built against a
// nonexistent state, if fighter.StateNo is not present in states.
func NewFighterRuntime(fighter match.FighterState, states map[int]cns.StateDef) (FighterRuntime, error) {
	def, ok := states[fighter.StateNo]
	if !ok {
		return FighterRuntime{}, fmt.Errorf("engine: NewFighterRuntime: state %d not found in loaded character", fighter.StateNo)
	}

	anim := def.Anim
	if anim == 0 {
		anim = fighter.StateNo
	}

	return FighterRuntime{
		Context: evaluator.Context{
			FighterState: fighter,
			Anim:         anim,
			Ctrl:         def.Ctrl,
		},
	}, nil
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
// each fighter, input recognition, CNS state-machine execution, and
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
	p1Out, err := tickFighter(programs[match.SideP1], runtimes[match.SideP1], state.Fighter(match.SideP1), inputs[match.SideP1], cfg.Tick)
	if err != nil {
		return TickResult{}, fmt.Errorf("engine: Tick: side %v: %w", match.SideP1, err)
	}
	p2Out, err := tickFighter(programs[match.SideP2], runtimes[match.SideP2], state.Fighter(match.SideP2), inputs[match.SideP2], cfg.Tick)
	if err != nil {
		return TickResult{}, fmt.Errorf("engine: Tick: side %v: %w", match.SideP2, err)
	}

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
		applyHitReaction(&programs, &runtimesOut, match.SideP1, p1Out.HitDef.Parameters, hit)
	}
	if p2Out.HasHitDef {
		damage := combat.DamageParams{Damage: p2Out.HitDef.Parameters["damage"]}
		newState, newCombo, hit := combat.ApplyHits(state, events, match.SideP2, damage, runtimesOut[match.SideP2].Combo, cfg.Tick, cfg.ComboWindow)
		state = newState
		runtimesOut[match.SideP2].Combo = newCombo
		applyHitReaction(&programs, &runtimesOut, match.SideP2, p2Out.HitDef.Parameters, hit)
	}

	p1Fighter := state.Fighter(match.SideP1)
	p1Fighter.StateNo = runtimesOut[match.SideP1].Context.StateNo
	p1Fighter.Power = runtimesOut[match.SideP1].Context.Power
	updatedP1 := p1Fighter
	if !p1Out.Frozen {
		updatedP1, err = physics.Step(p1Fighter, cfg.Bounds, cfg.Gravity)
		if err != nil {
			return TickResult{}, fmt.Errorf("engine: Tick: side %v: physics: %w", match.SideP1, err)
		}
	}
	state.Fighters[match.SideP1] = updatedP1

	p2Fighter := state.Fighter(match.SideP2)
	p2Fighter.StateNo = runtimesOut[match.SideP2].Context.StateNo
	p2Fighter.Power = runtimesOut[match.SideP2].Context.Power
	updatedP2 := p2Fighter
	if !p2Out.Frozen {
		updatedP2, err = physics.Step(p2Fighter, cfg.Bounds, cfg.Gravity)
		if err != nil {
			return TickResult{}, fmt.Errorf("engine: Tick: side %v: physics: %w", match.SideP2, err)
		}
	}
	state.Fighters[match.SideP2] = updatedP2

	var sounds [2][]evaluator.SoundEvent
	sounds[match.SideP1] = p1Out.Sounds
	sounds[match.SideP2] = p2Out.Sounds

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
		State:    state,
		Fighters: runtimesOut,
		Round:    result,
		Sounds:   sounds,
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
	// defender shake) this tick: no controllers ran, Time/AnimTime were
	// held, and Tick skips its physics.
	Frozen bool
}

// tickFighter advances one fighter's own per-tick simulation -- input
// recognition, CNS state-machine execution, and current-frame resolution
// -- given fighter, that fighter's live match.FighterState as of the start
// of this tick (position/facing/health, sourced from match.MatchState, the
// single source of truth Tick reconciles runtime.Context against every
// call).
func tickFighter(prog FighterProgram, runtime FighterRuntime, fighter match.FighterState, in input.TickInput, tick int) (fighterTickOutput, error) {
	ctx := runtime.Context
	ctx.FighterState = fighter

	newInputState, active := input.Step(runtime.Input, tick, fighter.Facing, in, prog.Commands)
	ctx.ActiveCommands = active

	// The frame resolved here is the one active as this tick begins --
	// i.e. before this tick's own state-machine transition (if any) is
	// applied. A transition's new animation takes effect starting next
	// tick, once AnimTime has actually reset -- see this item's own ADR.
	frame := currentFrame(findAnimation(prog, ctx.Anim), ctx.AnimTime)

	def, ok := prog.States[ctx.StateNo]
	if !ok {
		return fighterTickOutput{}, fmt.Errorf("current state %d not found in loaded character", ctx.StateNo)
	}

	// A fighter in hit pause (attacker freeze or defender shake) runs no
	// controllers, advances no Time/AnimTime and moves no physics; only
	// input recognition above keeps running. See .vibe/decisions/018.
	if runtime.HitPause > 0 || ctx.GetHit.HitShakeTime > 0 {
		frozenRuntime := FighterRuntime{Context: ctx, Input: newInputState, Combo: runtime.Combo, HitPause: runtime.HitPause}
		if frozenRuntime.HitPause > 0 {
			frozenRuntime.HitPause--
		}
		if frozenRuntime.Context.GetHit.HitShakeTime > 0 {
			frozenRuntime.Context.GetHit.HitShakeTime--
		}
		frozenRuntime.Context.Sounds = nil
		return fighterTickOutput{Runtime: frozenRuntime, Frame: frame, Frozen: true}, nil
	}

	result, err := statemachine.Step(ctx, prog.States)
	if err != nil {
		return fighterTickOutput{}, err
	}

	newCtx := result.Context
	if newCtx.StateNo != ctx.StateNo {
		// statemachine's own ChangeState handling already reset Time to 0
		// (character/cmd data plays no part in that); Anim/AnimTime/Ctrl
		// are enterState's responsibility, since statemachine has no
		// character-loadout knowledge of the target state's declared
		// values.
		enterState(&newCtx, prog.States[newCtx.StateNo])
	} else {
		newCtx.Time++
		newCtx.AnimTime++
	}
	// Once the shake is over the get-hit time counts down on every
	// unfrozen tick, stopping at -1 (HitOver).
	if newCtx.GetHit.Active && newCtx.GetHit.HitTime >= 0 {
		newCtx.GetHit.HitTime--
	}

	var hasHitDef bool
	var hitDef cns.Controller
	for _, i := range result.Applied {
		if strings.EqualFold(def.Controllers[i].Type, statemachine.ControllerTypeHitDef) {
			hitDef = def.Controllers[i]
			hasHitDef = true
			break
		}
	}

	return fighterTickOutput{
		Runtime:   FighterRuntime{Context: newCtx, Input: newInputState, Combo: runtime.Combo},
		Frame:     frame,
		HasHitDef: hasHitDef,
		HitDef:    hitDef,
		Sounds:    result.Sounds,
	}, nil
}

// enterState applies the bookkeeping of entering the state def to ctx,
// whose StateNo is already the new state's: Time and AnimTime restart at 0,
// Anim takes the StateDef's own value (defaulting to the state number,
// cns.StateDef.Anim's "0 means not set" convention) and Ctrl the StateDef's.
// Shared by tickFighter's ChangeState path and the get-hit entry below.
func enterState(ctx *evaluator.Context, def cns.StateDef) {
	ctx.Time = 0
	ctx.Anim = def.Anim
	if ctx.Anim == 0 {
		ctx.Anim = ctx.StateNo
	}
	ctx.AnimTime = 0
	ctx.Ctrl = def.Ctrl
}

// applyHitReaction applies a landed hit's HitDef to both fighters on the
// tick it lands (a no-op for a miss): the attacker is frozen for
// pausetime[0] ticks; the defender records its get-hit variables, is
// frozen for pausetime[1] ticks, and enters the get-hit state matching its
// current StateDef type (first letter, case-insensitive; omitted = S) --
// S 5000, C 5010, A 5020 -- in the same tick.
// A defender whose stance is L/U, or whose character has no such state,
// keeps its current state (the freeze and variables still apply), no
// error. See .vibe/decisions/018.
func applyHitReaction(programs *[2]FighterProgram, runtimes *[2]FighterRuntime, attacker match.Side, hitDefParams map[string]string, hit combat.HitResult) {
	if !hit.Hit {
		return
	}
	params := combat.ParseHitDef(hitDefParams)
	runtimes[attacker].HitPause = params.PauseTime[0]

	ctx := &runtimes[hit.Defender].Context
	states := programs[hit.Defender].States

	hitTime := params.GroundHitTime
	target := 0
	switch stanceLetter(states[ctx.StateNo].Type) {
	case 'S':
		target = 5000
	case 'C':
		target = 5010
	case 'A':
		target = 5020
		hitTime = params.AirHitTime
	}
	ctx.GetHit = evaluator.GetHit{
		Active:       true,
		HitShakeTime: params.PauseTime[1],
		HitTime:      hitTime,
		SlideTime:    params.GroundSlideTime,
		AnimType:     params.AnimType,
		GroundType:   params.GroundType,
		AirType:      params.AirType,
	}
	if def, ok := states[target]; ok && target != 0 {
		ctx.StateNo = target
		enterState(ctx, def)
	}
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

	elapsed := animTime
	i := 0
	for {
		f := anim.Frames[i]
		if f.Time <= 0 || elapsed < f.Time {
			return f
		}
		elapsed -= f.Time
		i++
		if i >= len(anim.Frames) {
			i = anim.LoopStart
			if i < 0 || i >= len(anim.Frames) {
				i = 0
			}
		}
	}
}
