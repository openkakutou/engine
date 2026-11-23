package engine

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/v2/combat"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/hitdetect"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
	"github.com/openkakutou/engine/v2/physics"
	"github.com/openkakutou/engine/v2/round"
	"github.com/openkakutou/engine/v2/statemachine"
	"github.com/openkakutou/engine/v2/zssexec"
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
	// IkemenVersion is the .def's [Info] ikemenversion, read as Ikemen GO's
	// ParseIkemenVersion does (common.go:931-945); all zero for a MUGEN
	// character. Not part of the JSON request shape.
	IkemenVersion [3]uint16 `json:"-"`
	// Name and Author are the .def [Info] name and author.
	Name, Author string `json:"-"`
	// MugenVersion is the .def's mugenversion as Ikemen GO's
	// ParseMugenVersion keeps it: 1.0 or 1.1, else 0.5 (WinMUGEN).
	MugenVersion [2]uint16 `json:"-"`

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
			// A parsed header that writes no optional field has a nil Written
			// (cns.StateDef): it omitted ctrl and anim, which then keep their
			// value; only a hand-built StateDef falls back to its typed fields.
			if def.Written == nil {
				def.Written = map[string]string{}
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
// either format. A state the program does not define gets the header of
// Ikemen GO's empty state: state type, move type and physics "unchanged"
// (ST_U, MT_U, char.go:6884-6887) and nothing else written.
func (p FighterProgram) stateHeader(number int) (entryHeader, bool) {
	if def, ok := p.States[number]; ok {
		return entryHeader{StateDef: def}, true
	}
	def, ok := p.script.Header(number)
	if !ok {
		return entryHeader{StateDef: emptyStateDef(number)}, false
	}
	return entryHeader{StateDef: def, script: true}, true
}

// emptyStateDef is the header of the empty state a change to state number
// enters when the character lacks it. Written is non-nil and empty so no
// field (ctrl, anim) reads as written.
func emptyStateDef(number int) cns.StateDef {
	return cns.StateDef{
		Number:   number,
		Type:     "U",
		MoveType: "U",
		Physics:  "U",
		Written:  map[string]string{},
	}
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
	Warnings     []string
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
		out := stateStep{Context: r.Context, Sounds: r.Sounds, Unsupported: r.Unsupported, Warnings: r.Warnings}
		controllers := p.States[number].Controllers
		for _, i := range r.Applied {
			// The last HitDef that ran wins (bytecode.go:8631-8650).
			if strings.EqualFold(controllers[i].Type, statemachine.ControllerTypeHitDef) {
				out.HitDef = controllers[i]
				out.HasHitDef = true
			}
		}
		return out, err
	}
	// An empty state has no controllers: it runs as nothing, as in Ikemen GO
	// (the state is a StateBytecode with no controllers, char.go:6884-6887).
	if !p.hasState(number) {
		return stateStep{Context: ctx}, nil
	}
	var r zssexec.Result
	var err error
	if frozen {
		r, err = zssexec.StepStateDuringHitPause(ctx, number, p.script, p.hasState)
	} else {
		r, err = zssexec.StepState(ctx, number, p.script, p.hasState)
	}
	return stateStep{Context: r.Context, HasHitDef: r.HasHitDef, HitDef: r.HitDef, Sounds: r.Sounds, Unsupported: r.Unsupported, SkippedCalls: r.SkippedCalls, Warnings: r.Warnings}, err
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
	// hit (HitDef pausetime[0]); each tick copies it to Context.HitPauseTime
	// for the HitPauseTime trigger. The defender's own freeze is
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
	// HitDef is the fighter's active HitDef, armed when its controller runs
	// and kept while Context.HitDefAttr is (a state change without
	// hitdefpersist clears both); HitDefHit is set once it has hit, as Ikemen
	// GO's hitdefTargets keeps a HitDef from hitting the same opponent twice
	// (char.go:869, 11193-11195).
	HitDef    cns.Controller
	HitDefHit bool
	// Projectiles are the fighter's live projectiles, its helpers' included.
	Projectiles []Projectile
	// Helpers are the player's helpers; NextHelper numbers them.
	Helpers    []Helper
	NextHelper int
	// Explods are the player's and its helpers' live explods.
	Explods []Explod
	// PauseMoveTime and SuperMoveTime are the ticks the fighter still moves
	// during a Pause and a SuperPause it started.
	PauseMoveTime, SuperMoveTime int
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
	ctx := evaluator.Context{FighterState: fighter, Consts: p.Constants, Anims: p.animationTable(), ScreenBound: true, MoveCamera: [2]bool{true, true}, AngleDrawScale: [2]float64{1, 1}}
	ctx.IkemenChar = p.IkemenVersion[0] != 0 || p.IkemenVersion[1] != 0
	ctx.Name, ctx.AuthorName = p.Name, p.Author
	ctx.MugenVersion = [2]int{int(p.MugenVersion[0]), int(p.MugenVersion[1])}
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
	// Events is each fighter's visual and feedback controllers of this tick
	// (PalFX, Explod, EnvShake, ...), in the order they ran, for the game to
	// draw; nil for a side with none.
	Events [2][]evaluator.ControllerEvent
	// Projectiles is each fighter's live projectiles after this tick, the
	// runtime's own slice.
	Projectiles [2][]Projectile
	// Helpers is each player's helpers after this tick, the runtime's own
	// slice.
	Helpers [2][]Helper
	// Explods is each player's live explods after this tick, the runtime's
	// own slice.
	Explods [2][]Explod
	// Warnings is each fighter's Ikemen GO console warnings of this tick, in
	// the order they happened: a state change to a state the character does
	// not have reads "changed to invalid state N (from state M)"
	// (char.go:6880-6887), and the fighter enters an empty state. nil for a
	// side with none; a fresh slice, allocated only when a warning happens.
	Warnings [2][]string
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
	Bounds *stage.StageBoundaries
	// Screen is the left and right screen edges of this tick (Ikemen GO
	// sys.xmin, sys.xmax, set from its camera), read by the edge distance
	// triggers. Left not below Right means unset: the stage bounds stand in.
	Screen [2]float64
	// Gravity is not read any more: each fighter falls by its own
	// movement.yaccel (Ikemen GO char.go:9963-9965). Kept so callers built
	// against earlier versions still compile.
	Gravity     float64
	Tick        int
	ComboWindow int
	// Round is where the round is (round.Clock); nil is the fight itself,
	// RoundState 2. The round timer counts and the outcome is decided only
	// in RoundState 2.
	Round *RoundProgress
}

// RoundProgress is the round's state for a tick: Ikemen GO's RoundState
// (0 to 4) and, once decided, its result.
type RoundProgress struct {
	State     int
	Result    round.RoundResult
	MatchOver bool
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
	setRoundProgress(&runtimes[match.SideP1].Context, cfg.Round)
	setRoundProgress(&runtimes[match.SideP2].Context, cfg.Round)
	runtimes[match.SideP1].Context.NumProjectiles = len(runtimes[match.SideP1].Projectiles)
	runtimes[match.SideP2].Context.NumProjectiles = len(runtimes[match.SideP2].Projectiles)
	runtimes[match.SideP1].Context.ProjectileIDs = projectileIDs(runtimes[match.SideP1].Projectiles)
	runtimes[match.SideP2].Context.ProjectileIDs = projectileIDs(runtimes[match.SideP2].Projectiles)
	p2Seen := snapshotOf(state.Fighter(match.SideP2), &runtimes[match.SideP2].Context)
	screen := screenEdges(cfg)
	paused := state.PauseTime > 0 || state.SuperTime > 0
	p1Out, err := tickOrFreeze(&state, fighterPrograms{programs[match.SideP1], programs[match.SideP2]}, runtimes[match.SideP1], state.Fighter(match.SideP1), p2Seen, inputs[match.SideP1], cfg.Tick, cfg.Bounds, screen, cfg.Gravity, state.Round, state.RandSeed)
	if err != nil {
		return TickResult{}, fmt.Errorf("engine: Tick: side %v: %w", match.SideP1, err)
	}
	// Both fighters draw Random from one sequence, P1 first, as Ikemen GO's
	// single seed is advanced in player order; the seed rides in match state.
	state.RandSeed = p1Out.Runtime.Context.RandSeed
	// P1's Target* controllers act on P2 before P2's tick.
	runtimes[match.SideP2].Context.FighterState = state.Fighters[match.SideP2]
	if err := applyTargetOps(&p1Out.Runtime.Context, &runtimes[match.SideP2].Context, programs[match.SideP1]); err != nil {
		return TickResult{}, fmt.Errorf("engine: Tick: side %v: %w", match.SideP1, err)
	}
	state.Fighters[match.SideP2] = runtimes[match.SideP2].Context.FighterState
	// What the controllers did to position, velocity, facing, health and
	// power is the fighter's new match state: hit detection below and the
	// next fighter's tick see it, as in Ikemen GO where a controller's
	// effect is immediate.
	state.Fighters[match.SideP1] = p1Out.Runtime.Context.FighterState
	p1Seen := snapshotOf(state.Fighter(match.SideP1), &p1Out.Runtime.Context)
	p2Out, err := tickOrFreeze(&state, fighterPrograms{programs[match.SideP2], programs[match.SideP1]}, runtimes[match.SideP2], state.Fighter(match.SideP2), p1Seen, inputs[match.SideP2], cfg.Tick, cfg.Bounds, screen, cfg.Gravity, state.Round, state.RandSeed)
	if err != nil {
		return TickResult{}, fmt.Errorf("engine: Tick: side %v: %w", match.SideP2, err)
	}

	state.RandSeed = p2Out.Runtime.Context.RandSeed
	state.Fighters[match.SideP2] = p2Out.Runtime.Context.FighterState
	// P2's Target* controllers act on P1, which has ticked.
	if err := applyTargetOps(&p2Out.Runtime.Context, &p1Out.Runtime.Context, programs[match.SideP2]); err != nil {
		return TickResult{}, fmt.Errorf("engine: Tick: side %v: %w", match.SideP2, err)
	}
	state.Fighters[match.SideP1] = p1Out.Runtime.Context.FighterState
	// BindToTarget moves P2 itself.
	state.Fighters[match.SideP2] = p2Out.Runtime.Context.FighterState
	// P2's helpers and explods see P1 after P2's Target* moved it, as P1's
	// see P2 after its tick: both sides at the same instant.
	p1Seen = snapshotOf(state.Fighter(match.SideP1), &p1Out.Runtime.Context)

	// The runtimes handed in stay as they were: the helpers and projectiles
	// change in fresh slices (nil stays nil, so a tick without any allocates
	// nothing).
	p1Out.Runtime.Helpers = slices.Clone(p1Out.Runtime.Helpers)
	p2Out.Runtime.Helpers = slices.Clone(p2Out.Runtime.Helpers)
	p1Out.Runtime.Projectiles = slices.Clone(p1Out.Runtime.Projectiles)
	p2Out.Runtime.Projectiles = slices.Clone(p2Out.Runtime.Projectiles)
	p1Out.Runtime.Explods = slices.Clone(p1Out.Runtime.Explods)
	p2Out.Runtime.Explods = slices.Clone(p2Out.Runtime.Explods)
	// Helpers act after both players, P1's first (charList creation order).
	p2Seen = snapshotOf(state.Fighter(match.SideP2), &p2Out.Runtime.Context)
	p1Helpers, err := tickHelpers(&state, &p1Out.Runtime, programs[match.SideP1], p2Seen, inputs[match.SideP1], cfg, screen, state.Round, &state.RandSeed)
	if err != nil {
		return TickResult{}, fmt.Errorf("engine: Tick: side %v: %w", match.SideP1, err)
	}
	p2Helpers, err := tickHelpers(&state, &p2Out.Runtime, programs[match.SideP2], p1Seen, inputs[match.SideP2], cfg, screen, state.Round, &state.RandSeed)
	if err != nil {
		return TickResult{}, fmt.Errorf("engine: Tick: side %v: %w", match.SideP2, err)
	}
	// Then the explods: the live ones update, the new ones appear.
	tickExplods(&p1Out.Runtime, programs[match.SideP1], p2Seen, screen, paused)
	tickExplods(&p2Out.Runtime, programs[match.SideP2], p1Seen, screen, paused)

	var frames [2]air.Frame
	frames[match.SideP1] = p1Out.Frame
	frames[match.SideP2] = p2Out.Frame
	// Ikemen GO pushes the fighters apart before hit detection, in
	// collisionDetection (char.go:14954-14960). See decision 031.
	pushFighters(&p1Out.Runtime.Context, &p2Out.Runtime.Context, frames, cfg.Bounds)
	if cfg.Screen[0] < cfg.Screen[1] {
		screenBound(&p1Out.Runtime.Context, cfg.Screen)
		screenBound(&p2Out.Runtime.Context, cfg.Screen)
	}
	state.Fighters[match.SideP1].Position = p1Out.Runtime.Context.Position
	state.Fighters[match.SideP2].Position = p2Out.Runtime.Context.Position
	events := hitdetect.Detect(&state, frames)
	events = hittableEvents(events, [2]*evaluator.Context{&p1Out.Runtime.Context, &p2Out.Runtime.Context})

	var runtimesOut [2]FighterRuntime
	runtimesOut[match.SideP1] = p1Out.Runtime
	runtimesOut[match.SideP2] = p2Out.Runtime

	armHitDef(&runtimesOut[match.SideP1], p1Out)
	armHitDef(&runtimesOut[match.SideP2], p2Out)
	// Projectiles spawned by this tick's controllers join the live ones, which
	// all move before collisions (Ikemen GO charUpdate, system.go:2715-2718).
	spawnProjectiles(&runtimesOut[match.SideP1].Context, &runtimesOut[match.SideP1])
	spawnProjectiles(&runtimesOut[match.SideP2].Context, &runtimesOut[match.SideP2])
	// Projectiles stand still through a pause.
	if !paused {
		updateProjectiles(&runtimesOut[match.SideP1], programs[match.SideP1], screen, cfg)
		updateProjectiles(&runtimesOut[match.SideP2], programs[match.SideP2], screen, cfg)
	}
	var hitSounds [2][]evaluator.SoundEvent
	// A ReversalDef beats the HitDef it reverses (char.go:11226-11240).
	if err := reverse(&state, frames, &programs, &runtimesOut, match.SideP1, cfg, &hitSounds); err != nil {
		return TickResult{}, err
	}
	if err := reverse(&state, frames, &programs, &runtimesOut, match.SideP2, cfg, &hitSounds); err != nil {
		return TickResult{}, err
	}
	if err := landHit(&state, events, &programs, &runtimesOut, match.SideP1, cfg, &hitSounds); err != nil {
		return TickResult{}, err
	}
	if err := landHit(&state, events, &programs, &runtimesOut, match.SideP2, cfg, &hitSounds); err != nil {
		return TickResult{}, err
	}
	if !paused {
		tradeProjectiles(&programs, &runtimesOut, cfg.Tick)
		if err := hitWithProjectiles(&state, &programs, &runtimesOut, match.SideP1, frames[match.SideP2].Clsn2, cfg, &hitSounds); err != nil {
			return TickResult{}, err
		}
		if err := hitWithProjectiles(&state, &programs, &runtimesOut, match.SideP2, frames[match.SideP1].Clsn2, cfg, &hitSounds); err != nil {
			return TickResult{}, err
		}
	}
	if err := hitWithHelpers(&state, &programs, &runtimesOut, match.SideP1, frames[match.SideP2].Clsn2, cfg, &hitSounds); err != nil {
		return TickResult{}, err
	}
	if err := hitWithHelpers(&state, &programs, &runtimesOut, match.SideP2, frames[match.SideP1].Clsn2, cfg, &hitSounds); err != nil {
		return TickResult{}, err
	}
	if !paused {
		tickProjectiles(&runtimesOut[match.SideP1])
		tickProjectiles(&runtimesOut[match.SideP2])
	}

	// What the hit phase did on the runtime contexts (get-hit or custom
	// state, p1facing/p2facing, the entered state's facep2 and velset, a
	// PowerAdd) is the fighters' new match state; strike kept each context's
	// Health equal to the state's.
	state.Fighters[match.SideP1] = runtimesOut[match.SideP1].Context.FighterState
	state.Fighters[match.SideP2] = runtimesOut[match.SideP2].Context.FighterState

	var sounds [2][]evaluator.SoundEvent
	sounds[match.SideP1] = append(append(p1Out.Sounds, p1Helpers.sounds...), hitSounds[match.SideP1]...)
	sounds[match.SideP2] = append(append(p2Out.Sounds, p2Helpers.sounds...), hitSounds[match.SideP2]...)

	var warnings [2][]string
	warnings[match.SideP1] = append(p1Out.Warnings, p1Helpers.warnings...)
	warnings[match.SideP2] = append(p2Out.Warnings, p2Helpers.warnings...)

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
	// The running pause counts down, then the pauses asked for this tick
	// start (in the next one).
	switch {
	case state.SuperTime > 0:
		state.SuperTime--
	case state.PauseTime > 0:
		state.PauseTime--
	}
	startPause(&state, &runtimesOut[match.SideP1])
	startPause(&state, &runtimesOut[match.SideP2])

	// Only the fight (RoundState 2) counts the time and decides the round.
	fighting := cfg.Round == nil || cfg.Round.State == 2
	untimed := state.RoundTimer <= 0 || !fighting || paused
	if !untimed {
		state.RoundTimer--
	}

	var result round.RoundResult
	if fighting {
		result = round.CheckOutcome(&state)
	}
	if untimed && (result.Outcome == round.OutcomeTimeout || result.Outcome == round.OutcomeTimeoutDraw) {
		result = round.RoundResult{}
	}

	return TickResult{
		State:        state,
		Fighters:     runtimesOut,
		Round:        result,
		Sounds:       sounds,
		Events:       [2][]evaluator.ControllerEvent{runtimesOut[match.SideP1].Context.Events, runtimesOut[match.SideP2].Context.Events},
		Projectiles:  [2][]Projectile{runtimesOut[match.SideP1].Projectiles, runtimesOut[match.SideP2].Projectiles},
		Helpers:      [2][]Helper{runtimesOut[match.SideP1].Helpers, runtimesOut[match.SideP2].Helpers},
		Explods:      [2][]Explod{runtimesOut[match.SideP1].Explods, runtimesOut[match.SideP2].Explods},
		Warnings:     warnings,
		Unsupported:  unsupported,
		SkippedCalls: skippedCalls,
	}, nil
}

// ranRuntime is the runtime after an unfrozen tick: the one that came in
// with this tick's context, input and reports, its hit pause over; every
// other field carries over.
func ranRuntime(runtime FighterRuntime, ctx evaluator.Context, in input.State, unsupported, skippedCalls []string) FighterRuntime {
	runtime.Context, runtime.Input = ctx, in
	runtime.Unsupported, runtime.SkippedCalls = unsupported, skippedCalls
	runtime.HitPause = 0
	return runtime
}

// fighterPrograms are the programs a fighter's tick reads: its own, and its
// opponent's, whose states it runs in a custom state (p2stateno,
// TargetState) and whose animation it shows after ChangeAnim2 (Ikemen GO's
// ss.sb.playerNo and animPN).
type fighterPrograms struct{ own, opp FighterProgram }

// states is the program whose states the fighter is running.
func (p fighterPrograms) states(ctx *evaluator.Context) FighterProgram {
	if ctx.InOpponentStates {
		return p.opp
	}
	return p.own
}

// anims is the program whose animation the fighter shows.
func (p fighterPrograms) anims(ctx *evaluator.Context) FighterProgram {
	if ctx.AnimFromOpponent {
		return p.opp
	}
	return p.own
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
	Warnings  []string
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
// inGuardState is Ikemen GO's inGuardState (char.go:9958-9961): the guard
// start, guarding, guard end and guard hit states.
func inGuardState(no int) bool {
	return no == 120 || no >= 130 && no <= 132 || no == 140 || no >= 150 && no <= 155
}

// canGuardNow is Ikemen GO's SCF_guard (char.go:12531-12537), set after the
// states run: idle with control (or landing, 52), or already guarding,
// holding back (or under autoguard), in a stance whose guard the states have
// not turned off.
func canGuardNow(ctx *evaluator.Context, d input.Directions) bool {
	idle := (ctx.Ctrl || ctx.StateNo == 52) && ctx.MoveType == 'I'
	if !idle && !inGuardState(ctx.StateNo) {
		return false
	}
	if !d.Back && !ctx.Asserted.Has("autoguard") {
		return false
	}
	switch ctx.StateType {
	case 'S':
		return !ctx.Asserted.Has("nostandguard")
	case 'C':
		return !ctx.Asserted.Has("nocrouchguard")
	case 'A':
		return !ctx.Asserted.Has("noairguard")
	}
	return false
}

// guards reports whether the defender guards an attack of params, Ikemen GO
// hitResultCheck (char.go:11466-11503): it can guard, is not already being
// hit by an unguarded hit, the attacker is not unguardable, the guardflag
// covers its stance, and it survives the guard damage.
func guards(attacker, defender *evaluator.Context, params combat.HitDefParams, guardDamage int) bool {
	if !defender.Guarding || attacker.Asserted.Has("unguardable") {
		return false
	}
	if defender.MoveType == 'H' && !defender.GetHit.Guarded {
		return false
	}
	covered := false
	switch defender.StateType {
	case 'S':
		covered = params.GuardFlag&combat.GuardHigh != 0
	case 'C':
		covered = params.GuardFlag&combat.GuardLow != 0
	case 'A':
		covered = params.GuardFlag&combat.GuardAir != 0
	}
	if !covered {
		return false
	}
	return defender.Health > guardChip(guardDamage, defender.Health, params) || defender.Asserted.Has("noko") || defender.Asserted.Has("noguardko")
}

// guardChip is the guard damage a defender of life takes: with guard.kill 0
// it stops at life - 1 (computeDamage's kill flag, char.go:8907-8915).
func guardChip(guardDamage, life int, params combat.HitDefParams) int {
	if !params.GuardKill && guardDamage >= life {
		return max(life-1, 0)
	}
	return guardDamage
}

// getUp is the engine's own end of a lie-down, Ikemen GO actionRun (char.go:
// 12573-12577, 12702-12704): a fighter lying in state 5110 for at least a
// tick whose down recover time ran out, alive and without
// nogetupfromliedown, enters 5120 and runs it; otherwise the time counts
// down while it lies there. Each hit sets the time (downRecoverTime).
func getUp(ctx *evaluator.Context, prog FighterProgram, runs *stateRuns) error {
	if ctx.StateNo != 5110 {
		return nil
	}
	if ctx.Time >= 1 && ctx.GetHit.DownRecoverTime <= 0 && ctx.Health > 0 && !ctx.Asserted.Has("nogetupfromliedown") && prog.hasState(5120) {
		ctx.PrevStateNo = ctx.StateNo
		ctx.StateNo = 5120
		ctx.StateChanged = true
		return runs.followChanges(fighterPrograms{prog, prog}, ctx, false)
	}
	if ctx.GetHit.DownRecoverTime > 0 {
		ctx.GetHit.DownRecoverTime--
	}
	return nil
}

// landHit lands attacker's armed HitDef on a contact this tick: the defender
// guards it when it can (guards), taking the guard damage and the guard
// reaction, else it takes the hit.
func landHit(state *match.MatchState, events []hitdetect.HitEvent, programs *[2]FighterProgram, runtimes *[2]FighterRuntime, attacker match.Side, cfg TickConfig, hitSounds *[2][]evaluator.SoundEvent) error {
	rt := &runtimes[attacker]
	if !hitDefReady(rt) || !touches(events, attacker) {
		return nil
	}
	hit, guarded, params := strike(state, runtimes, attacker, rt.HitDef, &rt.Context, cfg, hitSounds)
	rt.HitDefHit = hit.Hit
	if err := applyHitReaction(programs, runtimes, striker{side: attacker, rt: rt}, params, hit, guarded); err != nil {
		return fmt.Errorf("engine: Tick: side %v: %w", attacker, err)
	}
	return nil
}

// touches reports a contact of attacker's hit boxes this tick.
func touches(events []hitdetect.HitEvent, attacker match.Side) bool {
	for _, e := range events {
		if e.Attacker == attacker {
			return true
		}
	}
	return false
}

// hitSound is a HitDef's hitsound or guardsound, char.go:12100-12130: none
// by default; "S" reads the character's own sounds, no prefix or "F" the
// common sound file; played on the character's hit sound channel (-1).
// shortcut: hitsound.channel and the data.hitsound.channel constant are not
// read.
func hitSound(raw string, ctx *evaluator.Context) (evaluator.SoundEvent, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return evaluator.SoundEvent{}, false
	}
	common := true
	switch raw[0] {
	case 'S', 's':
		common, raw = false, raw[1:]
	case 'F', 'f':
		raw = raw[1:]
	}
	groupRaw, sampleRaw := raw, ""
	if parts := evaluator.SplitList(raw); len(parts) > 1 {
		groupRaw, sampleRaw = parts[0], parts[1]
	}
	group, err := evaluator.EvaluateMut(strings.TrimSpace(groupRaw), ctx)
	if err != nil || group.Int() < 0 {
		return evaluator.SoundEvent{}, false
	}
	sample := 0
	if strings.TrimSpace(sampleRaw) != "" {
		v, err := evaluator.EvaluateMut(strings.TrimSpace(sampleRaw), ctx)
		if err != nil || v.Int() < 0 {
			return evaluator.SoundEvent{}, false
		}
		sample = v.Int()
	}
	return evaluator.SoundEvent{Group: group.Int(), Sample: sample, Common: common, Channel: -1}, true
}

// hitSpark is the spark a landed (or guarded) hit shows, Ikemen GO hitspark
// (char.go:11313-11411), reported as a HitSpark event: anim is sparkno (or
// guard.sparkno) as written, else the attacker's data.sparkno (data.guard.
// sparkno); x is the target's edge facing the attacker less sparkxy's x
// toward the target, y the attacker's height plus sparkxy's y, in world space
// (Y up).
func hitSpark(attacker, defender *evaluator.Context, params map[string]string, guarded bool) evaluator.ControllerEvent {
	anim := strings.TrimSpace(params["sparkno"])
	if anim == "" {
		anim = strconv.Itoa(attacker.Consts.Lookup("data.sparkno").Int())
	}
	if guarded {
		anim = strings.TrimSpace(params["guard.sparkno"])
		if anim == "" {
			anim = strconv.Itoa(attacker.Consts.Lookup("data.guard.sparkno").Int())
		}
	}
	var offset [2]float64
	for i, part := range evaluator.SplitList(params["sparkxy"]) {
		if i > 1 {
			break
		}
		if v, err := evaluator.EvaluateMut(part, attacker); err == nil {
			offset[i] = v.Float()
		}
	}
	edge := defender.Consts.Lookup("size.ground.back").Number
	if attacker.Facing != defender.Facing {
		edge = defender.Consts.Lookup("size.ground.front").Number
	}
	defFacing, attFacing := facingSign(defender.Facing), facingSign(attacker.Facing)
	x := defender.Position.X + edge*defFacing - offset[0]*attFacing
	y := attacker.Position.Y - offset[1]
	return evaluator.ControllerEvent{Type: "HitSpark", Params: []evaluator.EventParam{
		{Name: "anim", Raw: anim},
		{Name: "x", Values: []float64{x}},
		{Name: "y", Values: []float64{y}},
	}}
}

// armHitDef keeps the fighter's active HitDef: a HitDef its states ran this
// tick with a valid attr replaces it and may hit again; a cleared
// HitDefAttr (a state change without hitdefpersist) disarms it.
func armHitDef(rt *FighterRuntime, out fighterTickOutput) {
	if rt.Context.HitDefAttr == 0 {
		rt.HitDef, rt.HitDefHit = cns.Controller{}, false
		return
	}
	if out.HasHitDef && !out.Frozen {
		rt.HitDef, rt.HitDefHit = out.HitDef, false
	}
}

// hitDefReady reports whether the fighter's HitDef can hit this tick: armed,
// not spent on a hit yet, and the fighter attacking (Ikemen GO hitActive,
// moveType not I, char.go:12984, 11168-11171).
func hitDefReady(rt *FighterRuntime) bool {
	return rt.HitDef.Type != "" && !rt.HitDefHit && rt.Context.MoveType != 'I'
}

// damageMultiplier is the attacker's attack over the defender's final
// defence, Ikemen GO's atkmul / finalDefense (char.go:8897, 12997-13001):
// [Data] attack and defence over 100, the defence times its DefenceMulSet
// multiplier unless that waits for a hit and the defender is not in one.
func damageMultiplier(attacker, defender *evaluator.Context) float64 {
	attack := attacker.Consts.Lookup("data.attack").Number / 100
	if attacker.HasAttackMul {
		attack *= attacker.AttackMul
	}
	custom := defender.CustomDefence
	if custom == 0 || (defender.DefenceMulOnHit && defender.MoveType != 'H') {
		custom = 1
	}
	defence := defender.Consts.Lookup("data.defence").Number / 100 * custom
	if defence == 0 {
		return attack
	}
	return attack / defence
}

// countDownHitBy counts each active HitBy slot down a tick, clearing it at 0
// (Ikemen GO char.go:12365-12371, before the fighter's states run).
func countDownHitBy(ctx *evaluator.Context) {
	for i := range ctx.HitBy {
		if ctx.HitBy[i].Time <= 0 {
			continue
		}
		ctx.HitBy[i].Time--
		if ctx.HitBy[i].Time == 0 {
			ctx.HitBy[i] = evaluator.HitBySlot{}
		}
	}
}

// hittableEvents drops, in place, the contacts whose defender's HitBy slots
// reject the attacker's HitDef (Ikemen GO checkHitByAllSlots, char.go:
// 10997-11042, in hit detection).
func hittableEvents(events []hitdetect.HitEvent, ctx [2]*evaluator.Context) []hitdetect.HitEvent {
	kept := events[:0]
	for _, e := range events {
		if evaluator.HittableBy(ctx[e.Defender].HitBy, ctx[e.Attacker].HitDefAttr) {
			kept = append(kept, e)
		}
	}
	return kept
}

// screenBound keeps a fighter between the screen edges unless a ScreenBound
// controller lifted it this tick, Ikemen GO's xScreenBound
// (char.go:10322-10336) with widthEdge 0, its default.
func screenBound(ctx *evaluator.Context, screen [2]float64) {
	if !ctx.ScreenBound {
		return
	}
	ctx.Position.X = min(max(ctx.Position.X, screen[0]), screen[1])
}

// screenEdges is cfg.Screen, or the stage bounds when the caller gives no
// screen (a far-away edge on each side without bounds either).
func screenEdges(cfg TickConfig) [2]float64 {
	if cfg.Screen[0] < cfg.Screen[1] {
		return cfg.Screen
	}
	if cfg.Bounds != nil {
		return [2]float64{float64(cfg.Bounds.Left), float64(cfg.Bounds.Right)}
	}
	return [2]float64{-math.MaxFloat32, math.MaxFloat32}
}

func tickFighter(progs fighterPrograms, runtime FighterRuntime, fighter match.FighterState, opponent evaluator.Opponent, in input.TickInput, tick int, bounds *stage.StageBoundaries, screen [2]float64, gravity float64, round int, seed int32) (fighterTickOutput, error) {
	ctx := runtime.Context
	ctx.ScreenLeft, ctx.ScreenRight = screen[0], screen[1]
	ctx.HitPauseTime = runtime.HitPause
	ctx.PosFreeze = false // a PosFreeze controller holds for the tick it runs
	ctx.Events = nil      // events are reported for the tick they happen in
	ctx.WidthEdge = [2]float64{}
	countDownHitBy(&ctx)
	ctx.FighterState = fighter
	ctx.P2 = opponent
	// A target that left its hit states is dropped before the fighter acts
	// (Ikemen GO char.go:12490, 10387-10404).
	if opponent.MoveType != 'H' {
		ctx.HasTarget = false
	}
	ctx.Round = round
	ctx.GameTime = tick
	ctx.RandSeed = seed
	ctx.YAccel = ctx.Consts.Lookup("movement.yaccel").Number
	prog := progs.own
	// The animation tables the triggers read: the fighter's own (set once
	// at its start when not precomputed) and the opponent's, which a
	// ChangeAnim2 shows, even one run later in this tick (built once when
	// not precomputed).
	if progs.own.animIndex != nil {
		ctx.Anims = progs.own.animIndex
	}
	if progs.opp.animIndex != nil {
		ctx.OpponentAnims = progs.opp.animIndex
	} else if ctx.OpponentAnims == nil {
		ctx.OpponentAnims = progs.opp.animationTable()
	}

	// The turn comes before the commands, so back and forward follow it the
	// same tick (system.go:13953-13958).
	if !ctx.IsHelper {
		autoTurn(&ctx, prog)
	}
	fighter.Facing = ctx.Facing
	newInputState, active := input.Step(runtime.Input, tick, fighter.Facing, in, prog.Commands)
	ctx.ActiveCommands = active

	// The frame resolved here is the one active as this tick begins --
	// i.e. before this tick's own state-machine transition (if any) is
	// applied. A transition's new animation takes effect starting next
	// tick, once AnimElapsed has actually reset -- see this item's own ADR.
	frame := currentFrame(findAnimation(progs.anims(&ctx), ctx.Anim), ctx.AnimElapsed)

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
	var warnings []string
	unsupported := runtime.Unsupported[:0]
	skippedCalls := runtime.SkippedCalls[:0]
	ctx.StateChanged = false
	// Before an unfrozen tick's states, Ikemen GO's actionPrepare turns
	// playerpush back on and resets the push properties (char.go:12350-
	// 12361): a PlayerPush controller lasts the tick it runs in.
	if !frozen {
		ctx.PlayerPush = true
		ctx.PushPriority = 0
		ctx.PushAffectTeam = 1
		ctx.ScreenBound, ctx.MoveCamera = true, [2]bool{true, true}
		ctx.Trans, ctx.TransAlpha = "", [2]int{}
		ctx.AngleDraw, ctx.AngleDrawScale = false, [2]float64{1, 1}
		ctx.DrawOffset = [2]float64{}
	}
	asserted := ctx.Asserted
	ctx.Asserted = evaluator.AssertFlags{} // the states assert them again (char.go:12430-12434)
	if !ctx.IsHelper {
		basicActions(&ctx, prog, in.Directions(fighter.Facing), asserted)
	}
	for _, number := range specialStates {
		// A helper runs one only with its keyctrl (char.go:12506-12527), and a
		// fighter in its opponent's states runs only its own -2.
		if !prog.hasState(number) || (ctx.IsHelper && !ctx.KeyCtrl[-1-number]) || (ctx.InOpponentStates && number != -2) {
			continue
		}
		special, err := prog.step(ctx, number, frozen)
		if err != nil {
			return fighterTickOutput{}, err
		}
		ctx = special.Context
		sounds = append(sounds, special.Sounds...)
		warnings = append(warnings, special.Warnings...)
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
		entered, _ := progs.states(&ctx).stateHeader(ctx.StateNo)
		if _, err := enterState(&ctx, entered); err != nil {
			return fighterTickOutput{}, err
		}
		// Ikemen GO applies the new animation in stateChange2, before
		// collision detection: this tick uses its first frame (char.go:6943,
		// 13049-13062).
		frame = currentFrame(findAnimation(progs.anims(&ctx), ctx.Anim), ctx.AnimElapsed)
	}

	result, err := progs.states(&ctx).step(ctx, ctx.StateNo, frozen)
	if err != nil {
		return fighterTickOutput{}, err
	}
	runs := stateRuns{sounds: sounds, warnings: warnings, unsupported: unsupported, skippedCalls: skippedCalls}
	newCtx := result.Context
	resolveAnimElem(&newCtx, findAnimation(progs.anims(&newCtx), newCtx.Anim))
	runs.add(result, &newCtx, frozen)
	// A ChangeState made by the current state enters the new state and runs
	// it in this same tick (Ikemen GO changeStateEx, char.go:6966-6994).
	if err := runs.followChanges(progs, &newCtx, frozen); err != nil {
		return fighterTickOutput{}, err
	}
	sounds, warnings, unsupported, skippedCalls = runs.sounds, runs.warnings, runs.unsupported, runs.skippedCalls
	if !frozen {
		if err := getUp(&newCtx, prog, &runs); err != nil {
			return fighterTickOutput{}, err
		}
		sounds, warnings, unsupported, skippedCalls = runs.sounds, runs.warnings, runs.unsupported, runs.skippedCalls
	}
	newCtx.Guarding = canGuardNow(&newCtx, in.Directions(fighter.Facing))
	if frozen {
		// The runtime as it came in, with this tick's context, input and
		// reports: every other field carries over.
		frozenRuntime := runtime
		frozenRuntime.Context, frozenRuntime.Input = newCtx, newInputState
		frozenRuntime.Unsupported, frozenRuntime.SkippedCalls = unsupported, skippedCalls
		if frozenRuntime.HitPause > 0 {
			frozenRuntime.HitPause--
		}
		if frozenRuntime.Context.GetHit.HitShakeTime > 0 {
			frozenRuntime.Context.GetHit.HitShakeTime--
		}
		if runtime.HitPause == 0 { // the defender's shake steps the fall time; an attacker's pause does not
			stepFall(&frozenRuntime.Context, false)
		}
		return fighterTickOutput{Runtime: frozenRuntime, Frame: frame, Frozen: true, Sounds: sounds, Warnings: warnings}, nil
	}
	// Ikemen GO actionRun moves the fighter after its states (posUpdate,
	// char.go:12583), then lands it (char.go:12584-12590).
	moved, err := physics.Step(newCtx.FighterState, bounds, gravityFor(&newCtx, newCtx.YAccel))
	if err != nil {
		return fighterTickOutput{}, fmt.Errorf("physics: %w", err)
	}
	if newCtx.PosFreeze {
		// CSF_posfreeze keeps the position; the velocity still changes
		// (char.go:10102-10105, then the physics types).
		moved.Position = newCtx.Position
	}
	newCtx.FighterState = moved
	applyFriction(&newCtx)
	if err := land(&newCtx, prog, &runs); err != nil {
		return fighterTickOutput{}, err
	}
	sounds, warnings, unsupported, skippedCalls = runs.sounds, runs.warnings, runs.unsupported, runs.skippedCalls
	// A state entered this tick shows its first frame to collision detection
	// (char.go:12608-12610); the animation steps after it (char.go:13055-13061).
	if runs.entered {
		frame = currentFrame(findAnimation(progs.anims(&newCtx), newCtx.Anim), newCtx.AnimElapsed)
	}
	// Time grows once, at the end of the tick, after the new state's run
	// (char.go:12592).
	newCtx.Time++
	newCtx.AnimElapsed++
	// After every unfrozen action, a state change included, the contact
	// counter grows (char.go:12594-12596); a state entered without
	// movehitpersist has just reset it to 0.
	if newCtx.MoveContactTime > 0 {
		newCtx.MoveContactTime++
	}
	if newCtx.MoveReversedTime > 0 {
		newCtx.MoveReversedTime++
	}
	stepFall(&newCtx, true)
	// A bound fighter stays bound while its opponent binds it again each
	// tick (TargetBind's time, 1 by default).
	if newCtx.GetHit.BindTime > 0 {
		newCtx.GetHit.BindTime--
	}
	// HitOverride slots last their time (-1 for ever).
	for i := range newCtx.HitOverrides {
		if newCtx.HitOverrides[i].Time > 0 {
			newCtx.HitOverrides[i].Time--
		}
	}
	// Once the shake is over the get-hit time counts down on every
	// unfrozen tick, stopping at -1 (HitOver).
	if newCtx.GetHit.Active && newCtx.GetHit.HitTime >= 0 {
		newCtx.GetHit.HitTime--
	}

	return fighterTickOutput{
		Runtime:   ranRuntime(runtime, newCtx, newInputState, unsupported, skippedCalls),
		Frame:     frame,
		HasHitDef: runs.hasHitDef,
		HitDef:    runs.hitDef,
		Sounds:    sounds,
		Warnings:  warnings,
	}, nil
}

// maxStateLoop is Ikemen GO's MaxLoop (bytecode.go:15): the most states one
// tick enters and runs after a ChangeState; the ChangeState of the last one
// is refused (stateChange1, char.go:6803-6807).
const maxStateLoop = 2500

// stateRuns collects what the states run in one tick produce, across the
// ChangeStates that run a new state at once.
type stateRuns struct {
	sounds       []evaluator.SoundEvent
	warnings     []string
	unsupported  []string
	skippedCalls []string
	hasHitDef    bool
	hitDef       cns.Controller
	entered      bool // a state was entered (not left pending) after the tick began
}

// add records the output of one state run. The HitDef that just applied
// becomes the one HitDefAttr reads, unless the state changed in the same run:
// entering the new state has just reset it (enterState, hitdefpersist).
func (r *stateRuns) add(step stateStep, ctx *evaluator.Context, frozen bool) {
	r.sounds = append(r.sounds, step.Sounds...)
	r.warnings = append(r.warnings, step.Warnings...)
	r.unsupported = appendMissing(r.unsupported, step.Unsupported)
	r.skippedCalls = appendMissing(r.skippedCalls, step.SkippedCalls)
	if !step.HasHitDef {
		return
	}
	// Its number parameters are evaluated now, as the HitDef runs (item 065);
	// the last HitDef run in the tick wins.
	r.hasHitDef, r.hitDef = true, resolveHitDef(step.HitDef, ctx)
	if frozen || ctx.StateChanged {
		return
	}
	if attr, err := evaluator.ParseAttr(step.HitDef.Parameters["attr"], true); err == nil {
		ctx.HitDefAttr = attr
		ctx.HitDefGuardDist = guardDist(r.hitDef, ctx)
	}
}

// guardDist is a HitDef's guard.dist x reach, front and back: its values,
// else the constants size.attack.dist.width front and back (Ikemen GO's
// HitDef defaults).
func guardDist(hitDef cns.Controller, ctx *evaluator.Context) [2]float64 {
	reach := [2]float64{
		ctx.Consts.Lookup("size.attack.dist.width.front").Number,
		ctx.Consts.Lookup("size.attack.dist.width.back").Number,
	}
	raw, ok := hitDef.Parameters["guard.dist"]
	if !ok {
		return reach
	}
	for i, part := range evaluator.SplitList(raw) {
		if i > 1 {
			break
		}
		if v, err := evaluator.EvaluateMut(part, ctx); err == nil {
			reach[i] = v.Number
		}
	}
	return reach
}

// followChanges enters the state a ChangeState just named and runs it, again
// for every ChangeState that run makes, up to maxStateLoop runs (Ikemen GO
// changeStateEx, char.go:6966-6994). During a hit pause the entry waits
// (char.go:6943): the state is only left pending, but its ignorehitpause
// controllers still run.
func (r *stateRuns) followChanges(progs fighterPrograms, ctx *evaluator.Context, frozen bool) error {
	for loops := 1; ctx.StateChanged; loops++ {
		ctx.StateChanged = false
		if frozen {
			ctx.StateEntryPending = true
		} else {
			ctx.StateEntryPending = false
			entered, _ := progs.states(ctx).stateHeader(ctx.StateNo)
			if _, err := enterState(ctx, entered); err != nil {
				return err
			}
			r.entered = true
		}
		stateNo, prevStateNo, time, persistent := ctx.StateNo, ctx.PrevStateNo, ctx.Time, ctx.Persistent
		step, err := progs.states(ctx).step(*ctx, ctx.StateNo, frozen)
		if err != nil {
			return err
		}
		*ctx = step.Context
		resolveAnimElem(ctx, findAnimation(progs.anims(ctx), ctx.Anim))
		r.add(step, ctx, frozen)
		if loops == maxStateLoop && ctx.StateChanged {
			ctx.StateNo, ctx.PrevStateNo, ctx.Time, ctx.Persistent = stateNo, prevStateNo, time, persistent
			ctx.StateChanged = false
		}
	}
	return nil
}

// land is Ikemen GO's hard-coded landing (char.go:12584-12590): a fighter in
// physics A falling (velocity down) at or below the ground, outside state
// 105, changes to state 52, entered and run at once like any ChangeState
// (changeStateEx, char.go:6987-6994): common1's state 52 puts it on the
// ground itself (common1.cns:273-281).
// A character without state 52 enters it empty, warned, as Ikemen GO does.
func land(ctx *evaluator.Context, prog FighterProgram, runs *stateRuns) error {
	if ctx.Physics != 'A' || ctx.Velocity.Y >= 0 || ctx.Position.Y > 0 || ctx.StateNo == 105 {
		return nil
	}
	warnIfMissing(ctx, prog, 52)
	ctx.PrevStateNo = ctx.StateNo
	ctx.StateNo = 52
	ctx.StateChanged = true
	return runs.followChanges(fighterPrograms{prog, prog}, ctx, false)
}

// autoTurn turns a fighter toward an opponent behind it, Ikemen GO's
// autoTurn (char.go:6731-6745) as its main loop calls it (system.go:
// 13953-13958): with control, in state 0, 11 or 20, or 52 on its first
// animation tick (any tick for an Ikemen GO character), unless the previous
// tick asserted noautoturn; standing it shows the turn animation 5,
// crouching 6.
// shortcut: the opponent's noturntarget, the stage's autoturn switch and a
// MUGEN character's turn on a ChangeState with control (char.go:6970-6974)
// are not done.
func autoTurn(ctx *evaluator.Context, prog FighterProgram) {
	if !ctx.Ctrl || ctx.Asserted.Has("noautoturn") || !ctx.P2.Valid {
		return
	}
	switch ctx.StateNo {
	case 0, 11, 20:
	case 52:
		if !ctx.IkemenChar {
			if t, err := evaluator.EvaluateMut("animtime", ctx); err != nil || t.Int() != 0 {
				return
			}
		}
	default:
		return
	}
	behind := ctx.P2.Position.X < ctx.Position.X
	if ctx.Facing == match.FacingLeft {
		behind = ctx.P2.Position.X > ctx.Position.X
	}
	if !behind {
		return
	}
	turnAnim := -1
	switch ctx.StateType {
	case 'S':
		turnAnim = 5
	case 'C':
		turnAnim = 6
	}
	if turnAnim >= 0 && ctx.Anim != turnAnim && hasAnimation(prog, turnAnim) {
		ctx.Anim, ctx.AnimElapsed = turnAnim, 0
		ctx.AnimElem, ctx.AnimElemTime = 0, 0
	}
	if ctx.Facing == match.FacingRight {
		ctx.Facing = match.FacingLeft
	} else {
		ctx.Facing = match.FacingRight
	}
}

// hasAnimation reports whether the character has animation number.
func hasAnimation(prog FighterProgram, number int) bool {
	a := findAnimation(prog, number)
	return a.Number == number && len(a.Frames) > 0
}

// basicActions are the walk, jump and crouch a fighter with control gets
// from the engine itself, before its states -3, -2 and -1 run (Ikemen GO
// actionPrepare, char.go:12300-12345; common1 has no ChangeState into 20 or
// 40). Like a ChangeState there, the change is entered after state -1.
// asserted are the AssertSpecial flags of the previous tick: nohardcodedkeys
// turns them all off, nojump, nocrouch, nostand, nowalk and nobrake one
// each.
// shortcut: there is no guard yet, so the guard entry (120) never fires and
// back does not cancel forward (char.go:12306, 12332); the air jump (45) and
// the round-over input lock (char.go:12309) are not done.
func basicActions(ctx *evaluator.Context, prog FighterProgram, d input.Directions, asserted evaluator.AssertFlags) {
	if asserted.Has("nohardcodedkeys") {
		return
	}
	// The guard entry comes first (char.go:12306): the fighter could guard
	// last tick, an attack is in reach and back is held.
	if ctx.Ctrl && ctx.Guarding && d.Back && !inGuardState(ctx.StateNo) && evaluator.InGuardDist(ctx) {
		basicChangeState(ctx, prog, 120)
		return
	}
	if ctx.Ctrl {
		switch {
		case ctx.StateType == 'S' && d.Up && !asserted.Has("nojump"):
			if ctx.StateNo != 40 {
				basicChangeState(ctx, prog, 40)
			}
		case ctx.StateType == 'S' && d.Down && !asserted.Has("nocrouch"):
			if ctx.StateNo != 10 {
				if ctx.StateNo != 100 {
					ctx.Velocity.X = 0
				}
				basicChangeState(ctx, prog, 10)
			}
		case ctx.StateType == 'C' && !d.Down && !asserted.Has("nostand"):
			if ctx.StateNo != 12 {
				basicChangeState(ctx, prog, 12)
			}
		case ctx.StateType == 'S' && d.Forward != d.Back && !asserted.Has("nowalk"):
			if ctx.StateNo != 20 {
				basicChangeState(ctx, prog, 20)
			}
		}
	}
	// Braking does not need control (char.go:12338-12342).
	if ctx.StateNo == 20 && d.Back == d.Forward && !asserted.Has("nobrake") {
		basicChangeState(ctx, prog, 0)
	}
}

// basicChangeState changes to state number as a ChangeState with no anim or
// ctrl does (statemachine applyChangeState); a character without that state
// enters it empty, with Ikemen GO's warning (char.go:6880-6887).
func basicChangeState(ctx *evaluator.Context, prog FighterProgram, number int) {
	warnIfMissing(ctx, prog, number)
	ctx.PrevStateNo = ctx.StateNo
	ctx.StateNo = number
	ctx.Time = 0
	ctx.StateChanged = true
	ctx.Persistent = [evaluator.MaxPersistent]int32{}
}

// warnIfMissing warns, as Ikemen GO's changeStateEx does (char.go:6880-
// 6887), when the fighter changes to a state its character lacks: it then
// runs that state empty.
func warnIfMissing(ctx *evaluator.Context, prog FighterProgram, number int) {
	if !prog.hasState(number) {
		ctx.Warnings = append(ctx.Warnings, fmt.Sprintf("changed to invalid state %d (from state %d)", number, ctx.StateNo))
	}
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
		ctx.MoveContactTime, ctx.MoveReversedTime = 0, 0
	}
	// and the hit count unless hitcountpersist (bytecode.go:5452-5455).
	if persist, err = headerBool(ctx, def, "hitcountpersist", def.HitCountPersist); err != nil {
		return false, err
	}
	if !persist {
		ctx.HitCount = 0
	}
	if persist, err = headerBool(ctx, def, "hitdefpersist", def.HitDefPersist); err != nil {
		return false, err
	}
	if !persist {
		ctx.HitDefAttr = 0
		ctx.ReversalAttr, ctx.Reversal = 0, nil
		ctx.HitDefGuardDist = [2]float64{}
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
	x, hasX, err := headerVelSet(ctx, def, "velset.x", 0)
	if err != nil {
		return false, err
	}
	if hasX {
		ctx.Velocity.X = 0 + x.Float()
	}
	if hasX && ctx.Facing == match.FacingLeft {
		ctx.Velocity.X = 0 - x.Float()
	}
	y, hasY, err := headerVelSet(ctx, def, "velset.y", 1)
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
	if !hasAnim && cnsWrote(def, "anim", def.Anim != 0) {
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
	if !hasCtrl && !def.script && cnsWrote(def, "ctrl", true) {
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
	ctx.Power = min(max(ctx.Power+add, 0), ctx.PowerMax())
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

// cnsWrote reports whether a .cns header writes field, from the written
// fields character records (cns.StateDef.Written): an omitted field keeps
// the fighter's current value (Ikemen GO bytecode.go:5444-5512). A StateDef
// built without them (Written nil, e.g. by hand) falls back to fallback.
func cnsWrote(def entryHeader, field string, fallback bool) bool {
	if def.Written == nil {
		return fallback
	}
	_, ok := def.Written[field]
	return ok
}

// headerVelSet is one component of the header's velset: a .zss header's
// expression under name, or the index-th comma-separated part of a .cns
// header's written "velset = x, y" (Ikemen GO compiler.go, commit 54d4148a),
// evaluated on entry.
func headerVelSet(ctx *evaluator.Context, def entryHeader, name string, index int) (v evaluator.Value, ok bool, err error) {
	if def.script {
		return headerExpr(ctx, def, name)
	}
	raw, ok := def.Written["velset"]
	if !ok {
		return evaluator.Value{}, false, nil
	}
	// x, y and an Ikemen GO z the engine does not read ("velset = 0,0,0").
	parts := evaluator.SplitList(raw)
	if index >= len(parts) {
		return evaluator.Value{}, false, nil
	}
	part := parts[index]
	if part == "" {
		return evaluator.Value{}, false, nil
	}
	v, err = evaluator.EvaluateMut(part, ctx)
	if err != nil {
		return evaluator.Value{}, false, fmt.Errorf("state %d header velset %q: %w", def.Number, raw, err)
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
		Valid:            true,
		FighterState:     fighter,
		Time:             ctx.Time,
		Anim:             ctx.Anim,
		PrevStateNo:      ctx.PrevStateNo,
		Ctrl:             ctx.Ctrl,
		StateType:        ctx.StateType,
		MoveType:         ctx.MoveType,
		GetHit:           ctx.GetHit,
		MoveContactTime:  ctx.MoveContactTime,
		HitDefAttr:       ctx.HitDefAttr,
		InCustomState:    ctx.InOpponentStates,
		ReversalAttr:     ctx.ReversalAttr,
		GuardDist:        ctx.HitDefGuardDist,
		Consts:           ctx.Consts,
		ScreenLeft:       ctx.ScreenLeft,
		ScreenRight:      ctx.ScreenRight,
		WidthEdge:        ctx.WidthEdge,
		IkemenChar:       ctx.IkemenChar,
		NumProjectiles:   ctx.NumProjectiles,
		Name:             ctx.Name,
		AuthorName:       ctx.AuthorName,
		IsHelper:         ctx.IsHelper,
		HelperID:         ctx.HelperID,
		Helpers:          ctx.Helpers,
		HitPauseTime:     ctx.HitPauseTime,
		AnimElapsed:      ctx.AnimElapsed,
		Anims:            ctx.Anims,
		OpponentAnims:    ctx.OpponentAnims,
		AnimFromOpponent: ctx.AnimFromOpponent,
		PalNo:            ctx.PalNo,
		Vars:             ctx.Vars,
		FVars:            ctx.FVars,
		SysVars:          ctx.SysVars,
	}
}

// stepFall is what a fighter's tick does to its fall: in a hit state the fall
// time counts while the fall flag is set, and the first tick out of one clears
// the flag (Ikemen GO char.go:12644-12673). canRecover reads the fall time.
// An acted tick's first in the bounce (5100) or the lie down from a trip
// (5070) counts a fall unless asserted nofallcount (char.go:12949-12957);
// out of a hit state the count is reset.
func stepFall(ctx *evaluator.Context, acted bool) {
	switch {
	case ctx.MoveType != 'H':
		ctx.GetHit.Fall = false
		ctx.GetHit.FallCount = 0
	case ctx.GetHit.Fall:
		ctx.GetHit.FallTime++
	}
	landed := ctx.StateNo == 5070 || ctx.StateNo == 5100
	if acted && landed && ctx.Time == 1 && !ctx.Asserted.Has("nofallcount") {
		ctx.GetHit.FallCount++
	}
}

// striker is who lands a hit, Ikemen GO's hitting char (hitResultCheck's c,
// char.go:11414): the player or the helper whose HitDef hit, or the owner
// of the projectile that hit.
type striker struct {
	side match.Side
	rt   *FighterRuntime
	// projectile is the projectile that hit, nil for a HitDef.
	projectile *Projectile
}

// knockbackFacing is the direction the defender is knocked toward (byf,
// char.go:11644-11651, 12168-12176): the projectile's facing, else the
// striker's own once p1facing has turned it.
func (s striker) knockbackFacing() match.Facing {
	if s.projectile != nil {
		return s.projectile.Facing
	}
	return s.rt.Context.Facing
}

// applyHitReaction applies a landed hit's HitDef to both fighters on the
// tick it lands (a no-op for a miss): the striker is frozen for
// pausetime[0] ticks; the defender records its get-hit variables, is
// frozen for pausetime[1] ticks, and enters the get-hit state matching its
// current StateDef type (first letter, case-insensitive; omitted = S) --
// S 5000, C 5010, A 5020 -- in the same tick. A projectile's owner gets
// neither the contact, the pause, the hit count nor p1facing: they are the
// projectile's (char.go:14525-14535), p1stateno is still the owner's.
// A defender whose stance is L/U, or whose character has no such state,
// keeps its current state (the freeze and variables still apply), no
// error. It returns the error of a get-hit header expression the defender's
// state entry could not evaluate. See .vibe/decisions/018.
func applyHitReaction(programs *[2]FighterProgram, runtimes *[2]FighterRuntime, by striker, params combat.HitDefParams, hit combat.HitResult, guarded bool) error {
	if !hit.Hit {
		return nil
	}
	attackerCtx := &by.rt.Context
	if by.projectile == nil {
		// The contact counter starts at 1 on the tick of the hit (Ikemen GO
		// char.go:14270-14271, 13125-13126), and the defender becomes a
		// target.
		attackerCtx.MoveContactTime = 1
		attackerCtx.ContactGuarded = guarded
		attackerCtx.HasTarget = true
		// A hit adds to the attacker's hit count (char.go:13125-13129).
		if !guarded {
			attackerCtx.HitCount++
		}
	}

	ctx := &runtimes[hit.Defender].Context
	// The defender's hits taken in the combo, counted while it stays hit,
	// and the damage of this one (ghv.hitcount, ghv.damage).
	hitCount := 1
	if ctx.MoveType == 'H' {
		hitCount = ctx.GetHit.HitCount + 1
	}
	defer func() {
		ctx.GetHit.HitCount = hitCount
		ctx.GetHit.Damage = hit.HealthBefore - hit.HealthAfter
		ctx.GetHit.Attr = params.Attr
	}()
	defender := programs[hit.Defender]
	if guarded {
		if by.projectile == nil {
			by.rt.HitPause = params.GuardPauseTime[0]
		}
		return guardReaction(ctx, defender, params, by.knockbackFacing())
	}
	if by.projectile == nil {
		by.rt.HitPause = params.PauseTime[0]
	}

	// p1facing and p2facing turn the fighters (char.go:12194-12209): 1
	// faces the defender to the attacker, -1 turns.
	switch params.P2Facing {
	case 1:
		ctx.Facing = 1 - by.knockbackFacing()
	case -1:
		ctx.Facing = by.knockbackFacing()
	}
	if params.P1Facing < 0 && by.projectile == nil {
		attackerCtx.Facing = 1 - attackerCtx.Facing
	}
	if params.P1StateNo >= 0 {
		if err := programs[by.side].selfState(by.rt, params.P1StateNo); err != nil {
			return err
		}
	}

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
	ctx.GetHit = getHitFor(params, ctx, hitTime, by.knockbackFacing())
	// Ikemen GO makes the defender's move type H before entering the get-hit
	// state, whose StateDef may then set its own (char.go:13123-13129).
	ctx.MoveType = 'H'
	// A HitOverride catching the hit sends the defender to its own state
	// instead (or keeps it where it is), p2stateno ignored (char.go:11540-
	// 11600, 13205-13212).
	for _, o := range ctx.HitOverrides {
		if o.Time == 0 || !evaluator.OverrideMatches(o.Attr, params.Attr, ctx.StateType) {
			continue
		}
		if o.KeepState || o.StateNo < 0 {
			return nil
		}
		return defender.selfState(&runtimes[hit.Defender], o.StateNo)
	}
	// p2stateno sends the defender into the attacker's state of that number
	// instead of its get-hit state: a custom state (char.go:13176-13197).
	if params.P2StateNo >= 0 {
		if def, ok := programs[by.side].stateHeader(params.P2StateNo); ok {
			ctx.PrevStateNo, ctx.StateNo = ctx.StateNo, params.P2StateNo
			ctx.InOpponentStates = true
			ctx.Ctrl = false
			_, err := enterState(ctx, def)
			return err
		}
	}
	if target != 0 {
		// A character without the get-hit state enters it empty, warned
		// (char.go:6880-6887).
		def, _ := defender.stateHeader(target)
		warnIfMissing(ctx, defender, target)
		ctx.PrevStateNo = ctx.StateNo
		ctx.StateNo = target
		// The get-hit change passes ctrl 0 (changeStateEx(5000, pn, -1, 0),
		// char.go:13176-13197), which removes control (char.go:6981-6982).
		ctx.Ctrl = false
		if _, err := enterState(ctx, def); err != nil {
			return err
		}
	}
	return nil
}

// guardReaction is a guarded hit's effect on the defender, Ikemen GO
// char.go:11710-11729 and 13163-13173: the guard pause, hit, slide and
// control times (chained by character version, char.go:935-946), the guard
// velocity, no fall, and the guard hit state for its stance (150 standing,
// 152 crouching, 154 in the air) entered without control.
func guardReaction(ctx *evaluator.Context, defender FighterProgram, params combat.HitDefParams, attackerFacing match.Facing) error {
	hitTime := params.GuardHitTime
	if hitTime < 0 {
		hitTime = params.GroundSlideTime // MUGEN characters (char.go:937-938)
		if ctx.IkemenChar {
			hitTime = params.GroundHitTime
		}
	}
	slideTime := params.GuardSlideTime
	if slideTime < 0 {
		slideTime = hitTime
	}
	ctrlTime := params.GuardCtrlTime
	if ctrlTime < 0 {
		ctrlTime = slideTime
	}
	byf := 1.0
	if attackerFacing == match.FacingLeft {
		byf = -1
	}
	g := evaluator.GetHit{
		Active: true, Guarded: true,
		HitShakeTime: params.GuardPauseTime[1], HitTime: hitTime, SlideTime: slideTime, CtrlTime: ctrlTime,
		GroundType: params.GroundType, AirType: params.AirType, Yaccel: params.YAccel,
		DownRecoverTime: ctx.GetHit.DownRecoverTime,
		Xvel:            -params.GuardVelocityX * byf,
	}
	target := 150
	switch ctx.StateType {
	case 'C':
		target = 152
	case 'A':
		target = 154
		g.Xvel, g.Yvel = -params.AirGuardVelocityX*byf, params.AirGuardVelocityY
		if params.AirGuardCtrlTime >= 0 {
			g.CtrlTime = params.AirGuardCtrlTime
		}
	}
	ctx.GetHit = g
	def, ok := defender.stateHeader(target)
	if !ok {
		return nil
	}
	ctx.PrevStateNo = ctx.StateNo
	ctx.StateNo = target
	ctx.Ctrl = false
	_, err := enterState(ctx, def)
	return err
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
func getHitFor(params combat.HitDefParams, defender *evaluator.Context, hitTime int, attackerFacing match.Facing) evaluator.GetHit {
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
		DownRecoverTime: downRecoverTime(params, defender),
		HitID:           params.HitID,
		Fall:            defender.GetHit.Fall,
		FallXvel:        params.FallXVelocity,
		HasFallXvel:     params.HasFallXVelocity,
		FallDamage:      params.FallDamage,
		FallKill:        params.FallKill,
		FallEnvShake: evaluator.EnvShake{
			Time: params.FallEnvShakeTime, Freq: params.FallEnvShakeFreq,
			Ampl: params.FallEnvShakeAmpl, Phase: params.FallEnvShakePhase,
		},
	}
	// ghv.xvel = velocity x * -attacker facing (char.go:11690-11793).
	byf := 1.0
	if attackerFacing == match.FacingLeft {
		byf = -1
	}
	airborne := defender.StateType == 'A'
	if airborne {
		g.Xvel = -params.AirVelocityX * byf
		g.Yvel = params.AirVelocityY
		g.Fall = g.Fall || params.AirFall
		g.CtrlTime = params.AirHitTime
	} else {
		g.Xvel = -params.GroundVelocityX * byf
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

// downRecoverTime is the lie-down time a hit gives (char.go:11823-11830):
// the HitDef's down.recovertime, else the defender's data.liedown.time, each
// only shortening the time left of a defender already lying down.
func downRecoverTime(params combat.HitDefParams, defender *evaluator.Context) int {
	t := params.DownRecoverTime
	if t < 0 {
		t = defender.Consts.Lookup("data.liedown.time").Int()
	}
	if defender.StateType == 'L' && defender.GetHit.DownRecoverTime < t {
		return defender.GetHit.DownRecoverTime
	}
	return t
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
// applyFriction slows a fighter on the ground after it moved, Ikemen GO
// posUpdate (char.go:10121-10137): physics S multiplies the x velocity by
// the character's stand.friction and zeroes it under 1, physics C by its
// crouch.friction.
// shortcut: a HitDef's own ground friction for the fighter it hits
// (ghv.stand_friction, Ikemen GO only) is not read.
func applyFriction(ctx *evaluator.Context) {
	switch ctx.Physics {
	case 'S':
		ctx.Velocity.X *= ctx.Consts.Lookup("movement.stand.friction").Number
		if math.Abs(ctx.Velocity.X) < 1 {
			ctx.Velocity.X = 0
		}
	case 'C':
		ctx.Velocity.X *= ctx.Consts.Lookup("movement.crouch.friction").Number
	}
}

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
