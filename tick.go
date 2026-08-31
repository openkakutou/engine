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
}

// NewFighterRuntime builds a fresh FighterRuntime for a fighter entering
// states[fs.StateNo] for the first time -- typically match start or a
// round reset (see round.ResetRound). Time and AnimTime start at 0, and
// Anim/Ctrl take the entered state's own declared values (Anim defaulting
// to the state number itself, matching cns.StateDef.Anim's own "0 means
// not set" convention) -- mirroring the bookkeeping Tick itself applies on
// every later state transition, so a fighter's very first state is not a
// special case for any later trigger relying on Anim/Ctrl.
//
// Returns a descriptive error, rather than a runtime built against a
// nonexistent state, if fs.StateNo is not present in states.
func NewFighterRuntime(fs match.FighterState, states map[int]cns.StateDef) (FighterRuntime, error) {
	def, ok := states[fs.StateNo]
	if !ok {
		return FighterRuntime{}, fmt.Errorf("engine: NewFighterRuntime: state %d not found in loaded character", fs.StateNo)
	}

	anim := def.Anim
	if anim == 0 {
		anim = fs.StateNo
	}

	return FighterRuntime{
		Context: evaluator.Context{
			FighterState: fs,
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
}

// Tick advances a full simulation tick for both fighters -- the one
// function tying every prior engine package into a usable match loop: for
// each fighter, input recognition, CNS state-machine execution, and
// current-animation-frame resolution; then, once for the pair, hit
// detection; then, per fighter whose state's HitDef triggered this tick,
// damage/combo resolution; then, per fighter, physics; then a round-outcome
// check. This is the function the WASM entrypoint (cmd/wasm) exposes for a
// mode-* game app to drive a match one tick at a time -- see this item's
// own ADR for the scope this function deliberately does and does not cover.
//
// programs is read-only character data, loaded once per fighter outside
// the simulation loop. runtimes is each fighter's evolving state as of the
// start of this tick (see FighterRuntime); inputs is this tick's raw
// device input per fighter. bounds is the current stage's horizontal
// boundaries; gravity and comboWindow are simulation constants the caller
// supplies (engine holds no default balance data of its own, matching
// physics.Step and combat.ApplyHits's own existing contracts). tick is the
// current simulation tick number (see input.Step).
//
// Returns a descriptive error, never a panic, if either fighter's current
// state is not present in its own loaded FighterProgram, or if physics
// rejects the given stage boundaries.
func Tick(
	state match.MatchState,
	programs [2]FighterProgram,
	runtimes [2]FighterRuntime,
	inputs [2]input.TickInput,
	bounds *stage.StageBoundaries,
	gravity float64,
	tick, comboWindow int,
) (TickResult, error) {
	p1Out, err := tickFighter(programs[match.SideP1], runtimes[match.SideP1], state.Fighter(match.SideP1), inputs[match.SideP1], tick)
	if err != nil {
		return TickResult{}, fmt.Errorf("engine: Tick: side %v: %w", match.SideP1, err)
	}
	p2Out, err := tickFighter(programs[match.SideP2], runtimes[match.SideP2], state.Fighter(match.SideP2), inputs[match.SideP2], tick)
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
		newState, newCombo, _ := combat.ApplyHits(state, events, match.SideP1, p1Out.HitDef, runtimesOut[match.SideP1].Combo, tick, comboWindow)
		state = newState
		runtimesOut[match.SideP1].Combo = newCombo
	}
	if p2Out.HasHitDef {
		newState, newCombo, _ := combat.ApplyHits(state, events, match.SideP2, p2Out.HitDef, runtimesOut[match.SideP2].Combo, tick, comboWindow)
		state = newState
		runtimesOut[match.SideP2].Combo = newCombo
	}

	p1Fs := state.Fighter(match.SideP1)
	p1Fs.StateNo = runtimesOut[match.SideP1].Context.StateNo
	updatedP1, err := physics.Step(p1Fs, bounds, gravity)
	if err != nil {
		return TickResult{}, fmt.Errorf("engine: Tick: side %v: physics: %w", match.SideP1, err)
	}
	state.Fighters[match.SideP1] = updatedP1

	p2Fs := state.Fighter(match.SideP2)
	p2Fs.StateNo = runtimesOut[match.SideP2].Context.StateNo
	updatedP2, err := physics.Step(p2Fs, bounds, gravity)
	if err != nil {
		return TickResult{}, fmt.Errorf("engine: Tick: side %v: physics: %w", match.SideP2, err)
	}
	state.Fighters[match.SideP2] = updatedP2

	return TickResult{
		State:    state,
		Fighters: runtimesOut,
		Round:    round.CheckOutcome(&state),
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
}

// tickFighter advances one fighter's own per-tick simulation -- input
// recognition, CNS state-machine execution, and current-frame resolution
// -- given fs, that fighter's live match.FighterState as of the start of
// this tick (position/facing/health, sourced from match.MatchState, the
// single source of truth Tick reconciles runtime.Context against every
// call).
func tickFighter(prog FighterProgram, runtime FighterRuntime, fs match.FighterState, in input.TickInput, tick int) (fighterTickOutput, error) {
	ctx := runtime.Context
	ctx.FighterState = fs

	newInputState, active := input.Step(runtime.Input, tick, fs.Facing, in, prog.Commands)
	ctx.ActiveCommands = active

	// The frame resolved here is the one active as this tick begins --
	// i.e. before this tick's own state-machine transition (if any) is
	// applied. A transition's new animation takes effect starting next
	// tick, once AnimTime has actually reset -- see this item's own ADR.
	frame := currentFrame(findAnimation(prog.Animations, ctx.Anim), ctx.AnimTime)

	def, ok := prog.States[ctx.StateNo]
	if !ok {
		return fighterTickOutput{}, fmt.Errorf("current state %d not found in loaded character", ctx.StateNo)
	}

	result, err := statemachine.Step(ctx, prog.States)
	if err != nil {
		return fighterTickOutput{}, err
	}

	newCtx := result.Context
	if newCtx.StateNo != ctx.StateNo {
		// statemachine's own ChangeState handling already reset Time to 0
		// (character/cmd data plays no part in that); Anim/AnimTime/Ctrl
		// are this function's own responsibility, since statemachine has
		// no character-loadout knowledge of the target state's declared
		// values.
		entered := prog.States[newCtx.StateNo]
		newCtx.Anim = entered.Anim
		if newCtx.Anim == 0 {
			newCtx.Anim = newCtx.StateNo
		}
		newCtx.AnimTime = 0
		newCtx.Ctrl = entered.Ctrl
	} else {
		newCtx.Time++
		newCtx.AnimTime++
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
	}, nil
}

// findAnimation returns the Animation in anims whose Number matches
// number, or the zero Animation (no frames) if none matches -- a fighter
// whose current Anim references data its own FighterProgram doesn't carry
// simply contributes no Clsn boxes for that tick, the same "no error path,
// only geometry over whatever's there" stance hitdetect.Detect itself
// already takes on a frame with no boxes at all.
func findAnimation(anims []air.Animation, number int) air.Animation {
	for _, a := range anims {
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
