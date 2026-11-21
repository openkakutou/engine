package input

import (
	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/engine/v2/match"
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
	Up    bool `json:"up"`
	Down  bool `json:"down"`
	Left  bool `json:"left"`
	Right bool `json:"right"`
	// Buttons is the set of currently-held button names (e.g. "a", "b"),
	// matching character/cmd.Command.Input's own lowercase button tokens.
	// A nil map reads every button as not held.
	Buttons map[string]bool `json:"buttons"`
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
	// progress is each command's, by its position in the command file:
	// commands sharing a name are alternatives, each with its own progress
	// (Ikemen GO keeps one Command per [Command] block, input.go).
	progress []commandProgress
	// prevDirs and prevButtons are the previous tick's resolved directions
	// and held buttons, to tell a press (held now, not before) from a hold.
	prevDirs    directionSet
	prevButtons map[string]bool
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
// sequence -- and otherwise keeps its progress, as Ikemen GO's
// Command.Step drops a partial sequence only when its time runs out
// (input.go:2566-2584). A step without "/" matches only on the tick its
// inputs become pressed; a "/" step matches while they are held
// (input.go:2621-2625).
func Step(state State, tick int, facing match.Facing, in TickInput, cmds cmd.CommandFile) (State, map[string]bool) {
	held := in.resolve(facing)

	next := State{progress: make([]commandProgress, len(cmds.Commands)), prevDirs: held}
	if len(in.Buttons) > 0 {
		// A copy: the caller may reuse its map for the next tick.
		next.prevButtons = make(map[string]bool, len(in.Buttons))
		for b, down := range in.Buttons {
			next.prevButtons[b] = down
		}
	}
	now := keys{dirs: held, buttons: in.Buttons, prevDirs: state.prevDirs, prevButtons: state.prevButtons}
	active := make(map[string]bool, len(cmds.Commands))

	for i, c := range cmds.Commands {
		steps := parseStepsCached(c.Input)
		next.progress[i] = commandProgress{matchedTick: -1}
		if len(steps) == 0 {
			continue
		}

		p := commandProgress{matchedTick: -1}
		if len(state.progress) == len(cmds.Commands) {
			p = state.progress[i]
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

		p = advance(p, steps, tick, now)

		next.progress[i] = p
		if p.matchedTick >= 0 && tick-p.matchedTick <= bufferWindow {
			active[c.Name] = true
		}
	}

	return next, active
}

// advance matches held/buttons against steps[p.step] (the next step this
// command's sequence expects) and returns p updated for this tick -- see
// Step's doc comment for the exact matching/reset rules.
func advance(p commandProgress, steps []step, tick int, now keys) commandProgress {
	if matches(steps[p.step], now) {
		return chain(commit(p, steps, tick, p.step), steps, tick, now)
	}
	if p.step > 0 && matches(steps[0], now) {
		// A wrong input doesn't erase an otherwise-valid fresh attempt
		// starting on this very tick.
		return chain(commit(p, steps, tick, 0), steps, tick, now)
	}
	return p
}

// chain matches, on the tick a direction step matched, the button steps
// that follow it, as Ikemen GO checks such "direction to button" steps in
// the same frame (input.go:2385-2407, IsDirToButton input.go:2031-2054):
// "F, c" completes when F and c are pressed together.
func chain(p commandProgress, steps []step, tick int, now keys) commandProgress {
	for p.step > 0 && dirToButton(steps[p.step-1], steps[p.step]) && matches(steps[p.step], now) {
		p = commit(p, steps, tick, p.step)
	}
	return p
}

// dirToButton reports whether next can match on the tick cur did: cur
// presses no button, next is not held, and they share no direction.
func dirToButton(cur, next step) bool {
	if next.hold || len(cur.buttons) > 0 {
		return false
	}
	return !(cur.hasDir && next.hasDir && cur.dirs == next.dirs)
}

// keys is one tick's resolved directions and held buttons, with the
// previous tick's, to tell a press from a hold.
type keys struct {
	dirs, prevDirs       directionSet
	buttons, prevButtons map[string]bool
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

// matches reports whether this tick's keys satisfy s: each of its inputs is
// held, and, for a step without "/", was not held on the previous tick
// (Ikemen GO input.go:2621-2625).
func matches(s step, now keys) bool {
	if s.hasDir {
		if !dirsHeld(s, now.dirs) {
			return false
		}
		if !s.hold && dirsHeld(s, now.prevDirs) {
			return false
		}
	}
	for _, b := range s.buttons {
		if !now.buttons[b] {
			return false
		}
		if !s.hold && now.prevButtons[b] {
			return false
		}
	}
	return true
}

// dirsHeld reports s's directions held in held: exactly, or, for a "$"
// step, each of them whatever else is held (input.go:901-960).
func dirsHeld(s step, held directionSet) bool {
	if !s.dollar {
		return s.dirs == held
	}
	return (!s.dirs.up || held.up) && (!s.dirs.down || held.down) &&
		(!s.dirs.forward || held.forward) && (!s.dirs.back || held.back)
}

// Directions are the directions a fighter holds, relative to its facing.
type Directions struct {
	Up, Down, Forward, Back bool
}

// Directions resolves in's raw directions against facing, opposing ones
// cancelling out as for command matching.
func (in TickInput) Directions(facing match.Facing) Directions {
	d := in.resolve(facing)
	return Directions{Up: d.up, Down: d.down, Forward: d.forward, Back: d.back}
}
