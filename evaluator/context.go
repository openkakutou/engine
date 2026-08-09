package evaluator

import "github.com/openkakutou/engine/match"

// maxVars and maxSysVars mirror MUGEN's fixed variable slot counts
// (var(0)..var(59), sysvar(0)..sysvar(4)) — an out-of-range index is a
// genuine authoring mistake, not a value the evaluator should invent a
// meaning for.
const (
	maxVars    = 60
	maxSysVars = 5
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
}
