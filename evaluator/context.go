package evaluator

import (
	"github.com/openkakutou/character/air"
	"github.com/openkakutou/engine/v2/match"
	"strings"
)

// maxVars, maxSysVars, maxFVars and maxSysFVars mirror MUGEN's fixed variable
// slot counts (var(0)..var(59), sysvar(0)..sysvar(4), fvar(0)..fvar(39),
// sysfvar(0)..sysfvar(4)) — an out-of-range index is a genuine authoring
// mistake, not a value the evaluator should invent a meaning for. (Ikemen GO
// keeps its variables in unbounded sparse maps, char.go cnsvar/cnsfvar; the
// fixed arrays keep Context allocation-free and MUGEN's limits.)
const (
	maxVars     = 60
	maxSysVars  = 5
	maxFVars    = 40
	maxSysFVars = 5
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
	// AnimElapsed is the number of simulation ticks elapsed since Anim
	// started playing.
	AnimElapsed int
	// AnimElem and AnimElemTime are a ChangeAnim controller's pending request
	// to start the new animation at element AnimElem (1-based, 0 meaning no
	// request) AnimElemTime ticks into it. The statemachine cannot resolve it
	// -- it has no animation data -- so the root package's Tick converts it
	// into AnimElapsed and clears it the same tick. Not persistent fighter
	// state.
	AnimElem, AnimElemTime int
	// Anims is the character's animations by number, read by AnimElem,
	// AnimElemTime, AnimElemNo, AnimExist and SelfAnimExist. Like Consts it
	// is set once per fighter by FighterProgram.NewRuntime and never written
	// after, so copying a Context copies one map header, not the table. Nil
	// means the character has no animation.
	Anims map[int]air.Animation
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
	// FVars and SysFVars are MUGEN's float variables, indexed by fvar(N) and
	// sysfvar(N), stored as float32 like Ikemen GO's (char.go cnsfvar).
	FVars    [maxFVars]float32
	SysFVars [maxSysFVars]float32
	// Consts is the character's constants table, read by Const(name). Set
	// once per fighter by FighterProgram.NewRuntime, never written after, so
	// copying a Context copies one pointer. Nil reads Ikemen GO's defaults.
	Consts *Constants
	// PalNo is the palette number the game chose for the fighter, read by the
	// PalNo trigger (Ikemen GO gi().palno, bytecode.go:2671). Zero, the
	// choice not made, reads as 1.
	PalNo int
	// Maps are the fighter's named variables (Ikemen GO c.mapArray), keyed by
	// lower-cased name: MapSet and MapAdd write them, map(name) reads them.
	// The map is shared by the copies of a Context, like the fighter it
	// belongs to; nil until the first write.
	Maps map[string]Value
	// HitBy are the fighter's HitBy / NotHitBy slots (Ikemen GO c.hitby,
	// char.go:3710): which attacks may hit it, each for Time more ticks.
	HitBy [8]HitBySlot
	// PosFreeze, set by the PosFreeze controller, keeps the fighter's
	// position this tick (Ikemen GO CSF_posfreeze, char.go:10102-10105); the
	// engine clears it at the start of each tick.
	PosFreeze bool
	// HitPauseTime is the attacker's remaining hit pause (Ikemen GO
	// c.hitPauseTime), copied from the runtime each tick.
	HitPauseTime int
	// ScreenLeft and ScreenRight are the screen edges the fighter moves
	// between this tick (Ikemen GO sys.xmin, sys.xmax), read by the edge
	// distance triggers.
	ScreenLeft, ScreenRight float64
	// Events are the visual and feedback controllers the fighter's states ran
	// this tick, cleared by the engine at the start of each tick.
	Events []ControllerEvent
	// SprPriority is the fighter's draw priority (SprPriority controller).
	SprPriority int
	// WidthEdge is the front and back width the Width controller gives the
	// fighter against the screen edges this tick (Ikemen GO widthEdge, reset
	// each tick, char.go:12401-12406).
	WidthEdge [2]float64
	// Guarding is Ikemen GO's SCF_guard: the fighter can guard this tick
	// (char.go:12531-12537), computed by the engine after its states run.
	Guarding bool
	// ContactGuarded is true when the attacker's last contact was guarded,
	// for MoveHit and MoveGuarded (Ikemen GO mctype MC_Guarded).
	ContactGuarded bool
	// Asserted are the AssertSpecial flags the fighter's states asserted
	// this tick; the engine clears them each tick after the built-in
	// actions read them (Ikemen GO char.go:12300-12345, 12430-12434).
	Asserted AssertFlags
	// CustomDefence is the DefenceMulSet multiplier of the fighter's defence
	// (Ikemen GO customDefense; 0 reads as 1). DefenceMulOnHit applies it
	// only while the fighter is being hit (defenseMulDelay).
	CustomDefence   float64
	DefenceMulOnHit bool
	// MugenVersion is the character's .def mugenversion as Ikemen GO keeps it
	// (1.0, 1.1, else 0.5), read by MugenVersion(major|minor).
	MugenVersion [2]int
	// IkemenChar is true for a character whose .def gives an ikemenversion
	// (Ikemen GO stWgi().ikemenver not 0.0), false for a MUGEN one: some
	// defaults differ between the two.
	IkemenChar bool
	// AILevel is the level of the CPU driving the fighter, 0 for a human
	// player (Ikemen GO sys.aiLevel[playerNo], 1 to 8), read by the AILevel
	// and AILevelF triggers. The game sets it; the engine never writes it.
	AILevel float64
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
	// Warnings lists the console warnings Ikemen GO would print during the
	// current Step call (today: "changed to invalid state N (from state M)").
	// A per-call log like Sounds: Step resets it to nil before running a
	// state and moves it into its Result, so nothing allocates on a call
	// with no warning.
	Warnings []string
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
	// PlayerPush, PushPriority and PushAffectTeam are Ikemen GO's playerpush
	// flag, pushPriority and pushAffectTeam (char.go:12350-12361): Tick sets
	// them to true, 0 and 1 before every unfrozen tick's states, and the
	// PlayerPush controller changes them for that tick. PushAffectTeam is 1
	// for enemies, -1 for the own team only, 0 for both.
	PlayerPush     bool
	PushPriority   int
	PushAffectTeam int
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
	// P2 is the opponent as it stood when this fighter's turn in the tick
	// began, set by Tick before the fighter acts: the opponent triggers and
	// the redirections to it read it. See Opponent.
	P2 Opponent
	// MoveContactTime is Ikemen GO's |mctime|: 0 until this fighter's HitDef
	// lands, then 1 from the tick it lands, growing by one after each unfrozen
	// tick and reset when a state is entered (unless its StateDef says
	// movehitpersist). MoveContact and MoveHit read it; MoveGuarded and
	// MoveReversed read 0 until guarding and ReversalDef exist.
	MoveContactTime int
	// HitDefAttr is the attr of the fighter's current HitDef, parsed by
	// ParseAttr; HitDefAttr reads it while the move type is A.
	HitDefAttr int32
	// HitDefGuardDist is the current HitDef's guard.dist x reach, front and
	// back (its constants' size.attack.dist.width when the HitDef gives
	// none), read by the opponent's InGuardDist.
	HitDefGuardDist [2]float64
	// StateChanged reports that a ChangeState ran since the caller last
	// cleared it, even into the same state number: Ikemen GO re-enters the
	// state either way (char.go changeStateEx, 6966-6994). Tick clears it
	// before each phase and enters the state when it is set.
	StateChanged bool
	// StateEntryPending is a state change made during a hit pause whose
	// entry (Ikemen GO stateChange2: the StateDef's anim, ctrl, types and
	// persist resets) waits until the pause ends (char.go:6943-6944,
	// applied at 12529 before the current state runs).
	StateEntryPending bool
	// HasTarget reports that the fighter hit the opponent and the opponent has
	// not left its hit states since: the fighter's target list, which in a
	// one-on-one match holds at most the opponent (NumTarget, "target,").
	HasTarget bool
	// Locals is the frame of local variables ($name) the expression runs
	// in; see LocalFrame. The zero value has none.
	Locals LocalFrame
	// Persistent holds the counters of the current state's .zss
	// persistent(N) blocks, one per block in source order, the way Ikemen GO
	// keeps them in c.ss.ps (char.go:6891-6903). zssexec runs them and
	// zeroes them on a state change; see .vibe/decisions/022.
	Persistent [MaxPersistent]int32
	// ZSS reports that the expressions being evaluated come from a .zss
	// script: EvaluateMut then short-circuits && and || as Ikemen GO's .zss
	// compiler does (compiler.go:8410, 6042-6072), where a .cns controller
	// does not. zssexec sets it for the length of a Step, so the
	// statemachine handlers it calls through ApplyController follow; it is
	// false in every Context Step returns.
	ZSS bool
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
	// CtrlTime, Yvel, Yaccel, Fall, FallYvel, FallRecover, FallRecoverTime,
	// FallTime and HitID are the values the HitDef gave the hit (Ikemen GO's
	// ghv.ctrltime, yvel, yaccel, fallflag, fall_yvelocity, fall_recover,
	// fall_recovertime, fall_time, hitid, char.go:11680-11810). Yvel, Yaccel
	// and FallYvel are in the content's units, Y down, as GetHitVar returns
	// them. FallTime counts the ticks spent in a hit state with Fall set.
	CtrlTime     int
	Yvel, Yaccel float64
	// Xvel is the knock-back x velocity in world space (Ikemen GO ghv.xvel:
	// the HitDef's x times minus the attacker's facing, char.go:11690-11793),
	// which HitVelSet x gives the fighter.
	Xvel     float64
	Fall     bool
	FallYvel float64
	// FallXvel is fall.xvelocity relative to the fighter's facing, used by
	// HitFallVel when HasFallXvel; FallDamage and FallKill are fall.damage
	// and fall.kill, applied by HitFallDamage (char.go:9572-9604).
	FallXvel    float64
	HasFallXvel bool
	// FallEnvShake is the HitDef\'s fall.envshake, which FallEnvShake reports
	// once (its Time then spent).
	FallEnvShake    EnvShake
	FallDamage      int
	FallKill        bool
	FallRecover     bool
	FallRecoverTime int
	// Guarded is true when the hit was guarded (Ikemen GO ghv.guarded).
	Guarded bool
	// DownRecoverTime is the ticks left lying down before the engine gets
	// the fighter up (Ikemen GO ghv.down_recovertime).
	DownRecoverTime int
	FallTime        int
	HitID           int
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
	// Common is true when the sample is read from the common sound file
	// (fight.def's common.snd: the F prefix, Ikemen GO src/char.go:6653-6663
	// and src/fightscreen.go:4978), false when it is read from the
	// character's own sounds (no prefix or the S prefix).
	Common bool `json:"common"`
	// Channel is PlaySnd's "channel" parameter, -1 when absent (Ikemen GO
	// newPlaySndParams, src/sound.go:1471). A sound on a channel >= 0 stops
	// the fighter's sound still playing on the same channel (SoundChannels.
	// Request, src/sound.go:1247-1263) unless LowPriority is set.
	Channel int `json:"channel"`
	// LowPriority is PlaySnd's "lowpriority" parameter: when set, the sound
	// does not play if the fighter's channel is still busy instead of
	// stopping it (src/sound.go:1253-1256).
	LowPriority bool `json:"lowPriority"`
}

// MapValue is the named variable name (any case), the zero int when unset.
func (c *Context) MapValue(name string) Value {
	return c.Maps[strings.ToLower(name)]
}

// SetMap stores v under name (any case).
func (c *Context) SetMap(name string, v Value) {
	if c.Maps == nil {
		c.Maps = make(map[string]Value)
	}
	c.Maps[strings.ToLower(name)] = v
}
