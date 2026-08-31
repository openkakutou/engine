// Package input reads raw per-tick device input (directions and button
// presses) and matches it, within a buffered time window, against
// character/cmd's .cmd command definitions -- so a state-machine
// controller's "Command" trigger (already wired to read
// evaluator.Context.ActiveCommands, see engine/evaluator) can resolve
// correctly. See the package doc comment on Step for the buffered matching
// contract itself; this file covers only the small .cmd command-string
// mini-language used to describe a single command's required input
// sequence.
package input

import "strings"

// directionSet is the set of directional inputs required (or, on a
// TickInput, currently held) at one tick. A diagonal is the simultaneous
// combination of its two axes (e.g. down-forward is down && forward), the
// same way a real 8-way stick/D-pad position is read -- there is no
// separate "diagonal" flag to keep in sync with its two components.
type directionSet struct {
	up, down, back, forward bool
}

// step is one element of a parsed command's required input sequence: an
// optional required direction (hasDir distinguishes "must be exactly this
// direction" from "direction is irrelevant to this step", since a
// button-only step like "a" must not care about stick position) plus zero
// or more button names that must all be held simultaneously.
type step struct {
	hasDir  bool
	dirs    directionSet
	buttons []string
}

// directionCodes maps every MUGEN/Ikemen GO direction token this package
// recognizes to the directionSet it requires. Matched case-sensitively,
// exactly as written (uppercase, per MUGEN convention) -- unlike buttons,
// direction tokens are never case-folded: a real .cmd command string can
// legitimately combine an uppercase direction with a same-letter lowercase
// button (e.g. "B" for back plus the "b" kick button both appear in "B+b"),
// so case is the only thing that tells them apart.
var directionCodes = map[string]directionSet{
	"U":  {up: true},
	"D":  {down: true},
	"B":  {back: true},
	"F":  {forward: true},
	"UB": {up: true, back: true},
	"UF": {up: true, forward: true},
	"DB": {down: true, back: true},
	"DF": {down: true, forward: true},
}

// commandModifiers are the MUGEN/Ikemen GO command-string prefix
// modifiers this package recognizes syntactically (release-detection "~",
// 4-way-guarantee "$", hold "/", and the strict-timing step separator
// ">") but does not implement semantically -- a step carrying one of them
// is matched the same as its unmodified form. Full modifier semantics are
// deferred; see .vibe/decisions/006-command-modifier-prefixes-recognized-not-implemented.md.
const commandModifiers = "~$/>"

// stepsCache memoizes parseSteps by its raw input string, so Step's
// per-tick, per-fighter call (see matcher.go) does not re-split/re-parse
// the same character-loaded, match-lifetime-static c.Input string on every
// simulation tick. Safe as a plain, unsynchronized map for the same
// single-threaded, no-goroutines reason engine/evaluator's own parseCache
// is (see .vibe/decisions/009-011).
var stepsCache = make(map[string][]step)

// parseStepsCached returns parseSteps(input), reusing a cached result for
// an input string already seen.
func parseStepsCached(input string) []step {
	if s, ok := stepsCache[input]; ok {
		return s
	}
	s := parseSteps(input)
	stepsCache[input] = s
	return s
}

// parseSteps splits a .cmd Command.Input string (e.g. "~D, DF, F, a") into
// its ordered sequence of required steps. Steps are comma-separated;
// within a step, "+" joins inputs that must be held simultaneously (e.g.
// "D+a"). A stray empty step (e.g. from a doubled or trailing comma) is
// dropped rather than producing an unmatchable step that would block the
// whole command from ever completing. An input with no recognizable
// tokens at all returns a nil slice.
func parseSteps(input string) []step {
	var steps []step
	for _, tok := range strings.Split(input, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		steps = append(steps, parseStep(tok))
	}
	return steps
}

// parseStep parses one comma-separated token of a command string into the
// step it requires.
func parseStep(tok string) step {
	tok = strings.TrimLeft(tok, commandModifiers)

	var s step
	for _, part := range strings.Split(tok, "+") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if dirs, ok := directionCodes[part]; ok {
			s.hasDir = true
			s.dirs.up = s.dirs.up || dirs.up
			s.dirs.down = s.dirs.down || dirs.down
			s.dirs.back = s.dirs.back || dirs.back
			s.dirs.forward = s.dirs.forward || dirs.forward
			continue
		}
		s.buttons = append(s.buttons, strings.ToLower(part))
	}
	return s
}
