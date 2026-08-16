package input

import (
	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/engine/match"
)

// TickInput is one simulation tick's raw device input: which of the four
// directions are currently held, and which named buttons are currently
// held. It is deliberately not facing-relative -- Left/Right are the raw
// physical directions a host/game-mode input source would report (a
// keyboard key, a gamepad D-pad) -- Step resolves them against the
// fighter's current facing internally, since .cmd command strings are
// themselves authored relative to facing (F/B), not absolute left/right.
// See .vibe/decisions/005-tickinput-is-raw-left-right-not-facing-relative.md.
//
// Its zero value (nothing held) is valid and usable.
type TickInput struct {
	Up, Down, Left, Right bool
	// Buttons is the set of currently-held button names (e.g. "a", "b"),
	// matching character/cmd.Command.Input's own lowercase button tokens.
	// A nil map reads every button as not held.
	Buttons map[string]bool
}

// resolve converts a raw TickInput into the facing-relative directionSet a
// parsed step is matched against, given the fighter's current facing.
func (in TickInput) resolve(facing match.Facing) directionSet {
	forward, back := in.Right, in.Left
	if facing == match.FacingLeft {
		forward, back = in.Left, in.Right
	}
	// A raw dpad/keyboard can report both directions on the same axis at
	// once (e.g. two opposing keys held together); that cancels out to
	// neutral on that axis rather than producing a nonsensical direction,
	// the same way a real analog stick can't physically be in two places.
	return directionSet{
		up:      in.Up && !in.Down,
		down:    in.Down && !in.Up,
		back:    back && !forward,
		forward: forward && !back,
	}
}

// commandProgress is one command's evolving recognition state, carried
// across ticks by the caller inside State -- this package holds no hidden
// state of its own, the same statelessness convention engine/statemachine
// already follows for evaluator.Context.
type commandProgress struct {
	// step is the index, into the command's parsed sequence, of the next
	// step that must be matched. 0 means no progress yet.
	step int
	// startTick is the tick at which the current partial sequence's step 0
	// last matched -- the reference point the recognition window (Time) is
	// measured from.
	startTick int
	// matchedTick is the tick at which the full sequence last completed;
	// -1 means it has never completed (or its buffer window has nothing
	// left to report -- State never needs to distinguish the two, since
	// both read as "not currently active").
	matchedTick int
}

// State is a command matcher's evolving per-command progress, threaded
// across ticks by the caller the same way evaluator.Context is threaded
// through statemachine.Step calls. Its zero value is valid and usable:
// every command starts with no progress and no recognized match.
type State struct {
	progress map[string]commandProgress
}

// Step advances every command defined in cmds by one simulation tick,
// given tick (the current simulation tick number, expected to increase by
// exactly 1 between successive calls for the same logical stream of
// input), the fighter's current facing (to resolve in's raw Left/Right
// against the command strings' facing-relative F/B), and the raw input
// itself. It returns the updated State to pass into the next tick's call,
// and the set of command names currently recognized -- directly assignable
// to evaluator.Context.ActiveCommands, which the "Command" trigger already
// reads (see engine/evaluator).
//
// A command is recognized once its full step sequence has matched, in
// order, within its declared recognition window (Command.Time, falling
// back to CommandFile.Defaults.Time when zero) measured from the tick its
// first step matched -- exceeding the window abandons that partial
// progress rather than leaving it pending forever. Once recognized, a
// command stays reported as active for its buffer window (Command.
// BufferTime, falling back to Defaults.BufferTime) after the completing
// tick, then stops. A tick that doesn't match a command's next expected
// step tries that same tick against the command's first step instead --
// so one wrong input doesn't require fully replaying an otherwise-correct
// sequence -- and resets to no progress if that doesn't match either.
func Step(state State, tick int, facing match.Facing, in TickInput, cmds cmd.CommandFile) (State, map[string]bool) {
	held := in.resolve(facing)

	next := State{progress: make(map[string]commandProgress, len(cmds.Commands))}
	active := make(map[string]bool, len(cmds.Commands))

	for _, c := range cmds.Commands {
		steps := parseSteps(c.Input)
		if len(steps) == 0 {
			continue
		}

		p, ok := state.progress[c.Name]
		if !ok {
			p = commandProgress{matchedTick: -1}
		}

		recognitionWindow := c.Time
		if recognitionWindow == 0 {
			recognitionWindow = cmds.Defaults.Time
		}
		bufferWindow := c.BufferTime
		if bufferWindow == 0 {
			bufferWindow = cmds.Defaults.BufferTime
		}

		if p.step > 0 && tick-p.startTick > recognitionWindow {
			p.step = 0
		}

		p = advance(p, steps, tick, held, in.Buttons)

		next.progress[c.Name] = p
		if p.matchedTick >= 0 && tick-p.matchedTick <= bufferWindow {
			active[c.Name] = true
		}
	}

	return next, active
}

// advance matches held/buttons against steps[p.step] (the next step this
// command's sequence expects) and returns p updated for this tick -- see
// Step's doc comment for the exact matching/reset rules.
func advance(p commandProgress, steps []step, tick int, held directionSet, buttons map[string]bool) commandProgress {
	if matches(steps[p.step], held, buttons) {
		return commit(p, steps, tick, p.step)
	}
	if p.step > 0 && matches(steps[0], held, buttons) {
		// A wrong input doesn't erase an otherwise-valid fresh attempt
		// starting on this very tick.
		return commit(p, steps, tick, 0)
	}
	if p.step > 0 {
		p.step = 0
	}
	return p
}

// commit records step matchedStep having matched on tick, advancing p's
// progress (and, if that completed the sequence, its recognized match).
func commit(p commandProgress, steps []step, tick, matchedStep int) commandProgress {
	if matchedStep == 0 {
		p.startTick = tick
	}
	p.step = matchedStep + 1
	if p.step == len(steps) {
		p.matchedTick = tick
		p.step = 0
	}
	return p
}

// matches reports whether the current tick's resolved direction and held
// buttons satisfy s.
func matches(s step, held directionSet, buttons map[string]bool) bool {
	if s.hasDir && s.dirs != held {
		return false
	}
	for _, b := range s.buttons {
		if !buttons[b] {
			return false
		}
	}
	return true
}
