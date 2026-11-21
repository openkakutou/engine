package engine

import (
	"strings"
	"testing"

	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/match"
)

// bytecode.go:5508-5510: a Statedef that omits ctrl keeps the fighter's.
// Ryu (SVC)'s crouch states write only type and physics, which the parser
// reports as a nil Written: the fighter must keep its control there.
func TestFromFiles_AHeaderWritingNoOptionalFieldKeepsControl(t *testing.T) {
	src := "[Statedef 0]\ntype = S\nctrl = 1\n\n[State 0, 1]\ntype = ChangeState\ntrigger1 = time = 0\nvalue = 10\n\n[Statedef 10]\ntype = C\nphysics = C\n"
	defs, err := cns.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	prog := NewFighterProgramFromFiles([]StateFile{{States: defs}}, nil, cmd.CommandFile{})
	state, progs, rts := projMatch(t, prog, 900)
	progs[0] = prog
	rt, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000})
	if err != nil {
		t.Fatal(err)
	}
	rts[0] = rt
	// One tick: on the next one, crouching without down goes on to 12.
	final, _ := runProj(t, state, progs, rts, 1)
	c := final.Fighters[match.SideP1].Context
	if c.StateNo != 10 || !c.Ctrl {
		t.Errorf("state %d ctrl %v, want 10 with control kept", c.StateNo, c.Ctrl)
	}
}

// Ikemen GO's velset takes a z (Spiderman "velset = 0,0,0"): the engine
// reads x and y and ignores it.
func TestFromFiles_AHeaderVelsetWithAZIsRead(t *testing.T) {
	src := "[Statedef 0]\ntype = S\nphysics = N\nvelset = 3, -2, 0\n"
	defs, err := cns.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	prog := NewFighterProgramFromFiles([]StateFile{{States: defs}}, nil, cmd.CommandFile{})
	rt, err := prog.NewRuntime(match.FighterState{Side: match.SideP1, Health: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if v := rt.Context.Velocity; v.X != 3 || v.Y != 2 {
		t.Errorf("velocity = %+v, want 3, 2 (y up)", v)
	}
}
