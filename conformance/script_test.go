package conformance

import (
	"reflect"
	"strings"
	"testing"

	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

func TestInputFor_SecondSide_RunsTheSameScriptOffsetByHalfItsLength(t *testing.T) {
	half := scriptLength() / 2
	for tick := 0; tick < scriptLength(); tick++ {
		p1 := inputFor(match.SideP1, tick+half, match.FacingRight)
		p2 := inputFor(match.SideP2, tick, match.FacingRight)
		if !reflect.DeepEqual(p1, p2) {
			t.Fatalf("tick %d: P2 input %+v differs from P1 at tick %d: %+v", tick, p2, tick+half, p1)
		}
	}
}

func TestInputFor_OverOneCycle_PressesEveryDirectionAndButton(t *testing.T) {
	seen := map[string]bool{}
	down := map[string]bool{}
	for tick := 0; tick < scriptLength(); tick++ {
		in := inputFor(match.SideP1, tick, match.FacingRight)
		for name, held := range map[string]bool{"up": in.Up, "down": in.Down, "forward": in.Right, "back": in.Left} {
			if held {
				seen[name] = true
			}
		}
		for _, b := range []string{"a", "b", "c", "x", "y", "z", "s"} {
			if in.Buttons[b] {
				seen[b] = true
				if in.Down {
					down[b] = true
				}
			}
		}
	}
	for _, name := range []string{"up", "down", "forward", "back", "a", "b", "c", "x", "y", "z", "s"} {
		if !seen[name] {
			t.Errorf("%q is never pressed in a cycle", name)
		}
	}
	if len(down) == 0 {
		t.Error("no button is ever pressed together with down")
	}
}

func TestInputFor_FacingLeft_MirrorsForwardAndBack(t *testing.T) {
	for tick := 0; tick < scriptLength(); tick++ {
		right := inputFor(match.SideP1, tick, match.FacingRight)
		left := inputFor(match.SideP1, tick, match.FacingLeft)
		if right.Right != left.Left || right.Left != left.Right {
			t.Fatalf("tick %d: facing left does not mirror forward/back", tick)
		}
	}
}

func TestInputFor_OverOneCycle_ExecutesMotionCommands(t *testing.T) {
	// The charge holds back ("/B"): a plain "B" step is one press, and a
	// press 45 ticks before F is outside the 20-tick window (Ikemen GO
	// input.go:2580-2584, 2621-2625).
	const cmdSrc = `[Command]
name = "qcf_a"
command = D, DF, F, a
time = 20

[Command]
name = "dp_b"
command = F, D, DF, b
time = 20

[Command]
name = "charge_c"
command = /B, F, c
time = 20
`
	cf, err := cmd.Parse(strings.NewReader(cmdSrc))
	if err != nil {
		t.Fatalf("cmd.Parse: %v", err)
	}
	for _, side := range []match.Side{match.SideP1, match.SideP2} {
		facing := match.FacingRight
		if side == match.SideP2 {
			facing = match.FacingLeft
		}
		fired := map[string]bool{}
		var state input.State
		for tick := 1; tick <= 2*scriptLength(); tick++ {
			var active map[string]bool
			state, active = input.Step(state, tick, facing, inputFor(side, tick-1, facing), cf)
			for name := range active {
				fired[name] = true
			}
		}
		for _, name := range []string{"qcf_a", "dp_b", "charge_c"} {
			if !fired[name] {
				t.Errorf("side %v: command %s never recognised over two cycles", side, name)
			}
		}
	}
}
