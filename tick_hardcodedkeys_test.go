package engine

import (
	"testing"

	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/v2/input"
)

// hardcodedKeysZss has the standing, crouching, walking and jump-start
// states and no state -1: only the engine's built-in basic actions (Ikemen GO
// char.go:12300-12345) can move the fighter between them.
const hardcodedKeysZss = `
[StateDef 0; type: S; physics: S;]
null{}

[StateDef 10; type: C; physics: C;]
null{}

[StateDef 12; type: S; physics: S;]
null{}

[StateDef 20; type: S; physics: S;]
null{}

[StateDef 40; type: S; physics: S; ctrl: 0;]
null{}

[StateDef 100; type: S; physics: S;]
null{}
`

func runHardcodedKeys(t *testing.T, source string, inputs ...input.TickInput) []int {
	t.Helper()
	prog := NewFighterProgram(nil, nil, cmd.CommandFile{}).WithScripts([]zss.Script{mustScript(t, source)})
	m := newE2EMatch(t, prog)
	var states []int
	for _, in := range inputs {
		if err := m.step(in); err != nil {
			t.Fatal(err)
		}
		stateNo, _, _ := m.p1()
		states = append(states, stateNo)
	}
	return states
}

func TestTick_HardcodedKeys_BasicActionsWithControl(t *testing.T) {
	right, up, down := input.TickInput{Right: true}, input.TickInput{Up: true}, input.TickInput{Down: true}
	for _, tc := range []struct {
		name   string
		inputs []input.TickInput
		want   []int
	}{
		// char.go:12331-12335: forward held in a standing state walks.
		{"forward walks", []input.TickInput{right, right}, []int{20, 20}},
		// char.go:12338-12342: walking without forward brakes to state 0.
		{"release brakes", []input.TickInput{right, {}}, []int{20, 0}},
		// char.go:12308-12312: up held in a standing state jumps.
		{"up jumps", []input.TickInput{up}, []int{40}},
		// char.go:12320-12326: down comes before walking in the chain.
		{"down crouches", []input.TickInput{{Down: true, Right: true}}, []int{10}},
		// char.go:12327-12330: a crouching state without down stands up.
		{"release stands up", []input.TickInput{down, {}}, []int{10, 12}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := runHardcodedKeys(t, hardcodedKeysZss, tc.inputs...)
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("states = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestTick_HardcodedKeys_NothingWithoutControl(t *testing.T) {
	// char.go:12305: the basic actions need ctrl.
	source := hardcodedKeysZss + `
[StateDef 5; type: S; physics: S; ctrl: 0;]
null{}
[StateDef -1]
if stateNo = 0 && time = 0 {
	changeState{value: 5}
}
`
	got := runHardcodedKeys(t, source, input.TickInput{}, input.TickInput{Right: true}, input.TickInput{Up: true})
	if got[1] != 5 || got[2] != 5 {
		t.Errorf("states = %v, want to stay in 5 without control", got)
	}
}
