package evaluator

import "github.com/openkakutou/engine/match"

// maxVars and maxSysVars mirror MUGEN's fixed variable slot counts
// (var(0)..var(59), sysvar(0)..sysvar(4)) — an out-of-range index is a
// genuine authoring mistake, not a value the evaluator should invent a
// meaning for.
const (
	maxVars    = 60
	maxSysVars = 5
	// MaxPersistent is the number of persistent(N) counters a Statedef can
	// hold; zssexec rejects a Statedef with more at parse time.
	MaxPersistent = 16
)

// Context is the per-fighter, per-tick data a trigger expression is
// evaluated against.
//
// It embeds match.FighterState (item 001) for the fields that package
// already models (e.g. StateNo), and adds the runtime data the built-in
// triggers in this item need but that FighterState does not carry yet:
// the state timer, animation/animation timer, control flag, general-
// purpose variables, and currently-recognized input commands. That data
// is rightfully owned by state-machine execution (item 003) and command
// matching (item 008); Context exists so this item does not have to wait
// for either, and so match.FighterState's already-closed API stays
// untouched. See .vibe/decisions/002.
//
// Its zero value is valid and usable: every numeric field reads as 0,
// Ctrl reads as false, and a nil ActiveCommands reads every command as
// not currently held — matching a fighter that has just entered state 0
// with no input yet.
type Context struct {
	match.FighterState

	// Time is the number of simulation ticks elapsed since the fighter
	// entered its current StateNo.
	Time int
	// Ctrl reports whether the fighter currently has control.
	Ctrl bool
	// Anim is the fighter's current animation number.
	Anim int
	// AnimTime is the number of simulation ticks elapsed since Anim
	// started playing.
	AnimTime int
	// AnimElem and AnimElemTime are a ChangeAnim controller's pending request
	// to start the new animation at element AnimElem (1-based, 0 meaning no
	// request) AnimElemTime ticks into it. The statemachine cannot resolve it
	// -- it has no animation data -- so the root package's Tick converts it
	// into AnimTime and clears it the same tick. Not persistent fighter
	// state.
	AnimElem, AnimElemTime int
	// YAccel is the vertical acceleration the Gravity controller adds to
	// the fighter's downward velocity each time it runs (Ikemen GO's
	// movement.yaccel constant). The caller supplies it from simulation
	// config, like Round and RandSeed; zero makes Gravity a no-op.
	YAccel float64
	// Vars holds MUGEN's general-purpose per-fighter integer variables,
	// indexed by var(N). An index never written reads as 0, matching
	// MUGEN's default.
	Vars [maxVars]int
	// SysVars holds MUGEN's system integer variables, indexed by
	// sysvar(N). An index never written reads as 0.
	SysVars [maxSysVars]int
	// ActiveCommands is the set of command names currently recognized as
	// held/triggered this tick, checked by the Command trigger. A nil map
	// reads every command as not active.
	ActiveCommands map[string]bool
	// Sounds is the list of SoundEvents a PlaySnd controller recorded
	// during the current statemachine.Step call, in trigger order. Unlike
	// Vars/Power, this is not persistent fighter state: it is a per-tick
	// event log that Step clears to nil before running a state's
	// controllers and again before returning, so it never carries over
	// from a previous simulation tick's Context (see
	// .vibe/decisions/017 in the engine repo). Lives here, rather than on
	// statemachine.Result alone, because ApplyController's only mutable
	// output channel is *Context.
	Sounds []SoundEvent
	// Unsupported lists, lower-cased and deduplicated, the controller types
	// whose trigger conditions held during the current statemachine.Step
	// call but which the engine does not implement. Like Sounds it is a
	// per-call log, never carried over between ticks: Step resets its
	// length to 0 before and after running a state's controllers. Unlike
	// Sounds, the emptied slice keeps its backing array so a fighter that
	// triggers unsupported controllers every tick does not allocate a new
	// one each time (see .vibe/decisions/017 and 020).
	Unsupported []string
	// StateType and MoveType are the fighter's current state type (S, C, A
	// or L) and move type (I, A or H) as upper-case letters, read by the
	// StateType and MoveType triggers. Zero until the fighter has entered a
	// state that declares one; a state that declares none leaves the
	// previous value (MUGEN's "unchanged").
	StateType, MoveType byte
	// Physics is the fighter's current physics type (S, C, A or N) as an
	// upper-case letter, set by the state it entered and overridable by
	// StateTypeSet. Tick applies gravity automatically only for A, as
	// Ikemen GO's char.go posUpdate does. Zero before any state is entered.
	Physics byte
	// PrevStateNo is the number of the state the fighter was in before its
	// current one (0 before any change).
	PrevStateNo int
	// Round is the number of the round being fought, read by RoundNo.
	Round int
	// RandSeed is the state of the match's random generator, which both
	// fighters share: Ikemen GO draws Random from one global seed. The
	// caller loads it from match state before evaluating a fighter and
	// stores it back afterwards; EvalMut advances it.
	RandSeed int32
	// GetHit is the hit the fighter most recently took, as the engine
	// recorded it when the hit landed (see .vibe/decisions/018). Its zero
	// value is "no hit taken yet".
	GetHit GetHit
	// Locals is the frame of local variables ($name) the expression runs
	// in; see LocalFrame. The zero value has none.
	Locals LocalFrame
	// Persistent holds the counters of the current state's .zss
	// persistent(N) blocks, one per block in source order, the way Ikemen GO
	// keeps them in c.ss.ps (char.go:6891-6903). zssexec runs them and
	// zeroes them on a state change; see .vibe/decisions/022.
	Persistent [MaxPersistent]int32
}

// LocalFrame is the local variables an expression can read as $name: the
// caller (zssexec) fixes Names at parse time and passes the window of its
// value stack holding the current frame, so Values[i] is the value of
// Names[i]. A slot never assigned holds the zero Value, which reads as 0
// as Ikemen GO's bvNone (bytecode.go:1417, 1551) does in arithmetic.
type LocalFrame struct {
	Names  []string
	Values []Value
}

// GetHit is a fighter's get-hit variables (MUGEN's GetHitVar set), read by
// the HitShakeOver/HitOver triggers and GetHitVar(). A plain value struct,
// no pointers, so Context stays cheap to copy on the per-tick hot path.
// AnimType and GroundType/AirType carry MUGEN's numeric codes (animtype
// Light=0, Medium=1, Hard=2, Back=3, Up=4, DiagUp=5; type None=0, High=1,
// Low=2, Trip=3).
type GetHit struct {
	// Active reports that a hit has been taken: false on the zero value,
	// where HitOver reads true (MUGEN initializes hittime to -1).
	Active bool
	// HitShakeTime is the remaining frozen ticks of the defender's shake.
	HitShakeTime int
	// HitTime counts down on each unfrozen tick after the shake, stopping
	// at -1; HitOver reads true once it is negative.
	HitTime    int
	SlideTime  int
	AnimType   int
	GroundType int
	AirType    int
}

// SoundEvent is one PlaySnd controller execution recorded onto a fighter's
// Context: the (Group, Sample) pair its "value" parameter resolved to,
// addressing a character's decoded sound the same way character.Sound
// itself is keyed. engine only reports which sample was triggered -- it
// never decodes or plays the audio itself (see .vibe/decisions/017).
type SoundEvent struct {
	// Group is the sound group index the triggering PlaySnd controller's
	// "value" parameter resolved to.
	Group int `json:"group"`
	// Sample is the sound sample index within Group the same "value"
	// parameter resolved to.
	Sample int `json:"sample"`
}
