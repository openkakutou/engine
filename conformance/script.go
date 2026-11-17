package conformance

import (
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
)

// scriptStep holds a facing-relative input for a number of ticks.
type scriptStep struct {
	ticks                   int
	up, down, forward, back bool
	buttons                 string
}

// inputScript is the deterministic built-in input script both fighters
// follow (never the CPU, never randomness): neutral, each direction, each
// button, down plus each button, then the motions quarter-circle forward +
// a, dragon punch + b and back-charge forward + c. Directions are relative
// to the fighter's facing. It is built once; its length is scriptLength.
var inputScript = buildScript()

func buildScript() []scriptStep {
	neutral := func(n int) scriptStep { return scriptStep{ticks: n} }
	var s []scriptStep
	s = append(s,
		neutral(10),
		scriptStep{ticks: 20, forward: true},
		neutral(5),
		scriptStep{ticks: 20, back: true},
		neutral(5),
		scriptStep{ticks: 15, down: true},
		neutral(5),
		scriptStep{ticks: 15, up: true},
		neutral(10),
	)
	for _, b := range []string{"a", "b", "c", "x", "y", "z", "s"} {
		s = append(s, scriptStep{ticks: 3, buttons: b}, neutral(12))
	}
	for _, b := range []string{"a", "b", "c", "x", "y", "z"} {
		s = append(s, scriptStep{ticks: 3, down: true, buttons: b}, neutral(12))
	}
	s = append(s,
		// quarter-circle forward + a
		// Each direction lasts one tick: the matcher advances one step per
		// tick and resets on a repeated direction.
		scriptStep{ticks: 1, down: true},
		scriptStep{ticks: 1, down: true, forward: true},
		scriptStep{ticks: 1, forward: true},
		scriptStep{ticks: 2, forward: true, buttons: "a"},
		neutral(15),
		// dragon punch + b
		scriptStep{ticks: 1, forward: true},
		scriptStep{ticks: 1, down: true},
		scriptStep{ticks: 1, down: true, forward: true},
		scriptStep{ticks: 2, down: true, forward: true, buttons: "b"},
		neutral(15),
		// back-charge forward + c
		scriptStep{ticks: 45, back: true},
		scriptStep{ticks: 2, forward: true, buttons: "c"},
		neutral(15),
	)
	return s
}

// scriptLength is the number of ticks in one cycle of the input script.
func scriptLength() int {
	n := 0
	for _, s := range inputScript {
		n += s.ticks
	}
	return n
}

// inputFor is the raw input the given side presses at the given tick
// (0-based; the script cycles). The second side runs the same script offset
// by half its length. Forward/back are turned into raw left/right using the
// fighter's current facing, as input.TickInput expects.
func inputFor(side match.Side, tick int, facing match.Facing) input.TickInput {
	length := scriptLength()
	pos := tick
	if side == match.SideP2 {
		pos += length / 2
	}
	pos %= length
	for _, s := range inputScript {
		if pos >= s.ticks {
			pos -= s.ticks
			continue
		}
		in := input.TickInput{Up: s.up, Down: s.down}
		if facing == match.FacingRight {
			in.Right, in.Left = s.forward, s.back
		} else {
			in.Left, in.Right = s.forward, s.back
		}
		if s.buttons != "" {
			in.Buttons = map[string]bool{s.buttons: true}
		}
		return in
	}
	return input.TickInput{}
}
