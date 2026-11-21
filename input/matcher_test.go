package input

import (
	"testing"

	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/engine/v2/match"
)

// qcfCommands mirrors character/cmd's own real sample.cmd fixture
// (backlog item 008's own "Done" example): a single "QCF_a" command --
// quarter-circle-forward plus the "a" punch button -- with a 15-tick
// recognition window and a 1-tick post-match buffer.
func qcfCommands() cmd.CommandFile {
	return cmd.CommandFile{
		Defaults: cmd.CommandDefaults{Time: 15, BufferTime: 1},
		Commands: []cmd.Command{
			{Name: "QCF_a", Input: "~D, DF, F, a"},
		},
	}
}

func TestStep_ExactQuarterCircleForwardSequence_RecognizesTheCommand(t *testing.T) {
	cmds := qcfCommands()
	s := State{}

	// Tick 0: D. Tick 1: DF. Tick 2: F. Tick 3: press "a" (neutral stick).
	ticks := []TickInput{
		{Down: true},
		{Down: true, Right: true},
		{Right: true},
		{Buttons: map[string]bool{"a": true}},
	}

	var active map[string]bool
	for i, in := range ticks {
		s, active = Step(s, i, match.FacingRight, in, cmds)
	}

	if !active["QCF_a"] {
		t.Errorf("QCF_a not recognized after the exact sequence completed; active = %+v", active)
	}
}

func TestStep_SequenceMissingTheFinalDirection_NeverRecognizesTheCommand(t *testing.T) {
	cmds := qcfCommands()
	s := State{}

	// D, DF, then straight to the button -- skips the required "F" step,
	// which is close to (but not) the real command.
	ticks := []TickInput{
		{Down: true},
		{Down: true, Right: true},
		{Buttons: map[string]bool{"a": true}},
	}

	var active map[string]bool
	for i, in := range ticks {
		s, active = Step(s, i, match.FacingRight, in, cmds)
	}

	if active["QCF_a"] {
		t.Errorf("QCF_a incorrectly recognized from a sequence missing its final direction step")
	}
}

func TestStep_SequenceCompletedAfterTheBufferWindowElapses_NeverRecognizesTheCommand(t *testing.T) {
	cmds := qcfCommands() // Defaults.Time = 15
	s := State{}

	s, _ = Step(s, 0, match.FacingRight, TickInput{Down: true}, cmds)

	// Nothing happens for longer than the 15-tick recognition window --
	// the partial progress must be abandoned, not left waiting forever.
	for tick := 1; tick <= 16; tick++ {
		s, _ = Step(s, tick, match.FacingRight, TickInput{}, cmds)
	}

	// Now finish the sequence -- too late, the window already elapsed.
	var active map[string]bool
	s, _ = Step(s, 17, match.FacingRight, TickInput{Down: true, Right: true}, cmds)
	s, _ = Step(s, 18, match.FacingRight, TickInput{Right: true}, cmds)
	_, active = Step(s, 19, match.FacingRight, TickInput{Buttons: map[string]bool{"a": true}}, cmds)

	if active["QCF_a"] {
		t.Errorf("QCF_a incorrectly recognized after its recognition window (%d ticks) had already elapsed", cmds.Defaults.Time)
	}
}

func TestStep_RawRightWithFighterFacingLeft_CountsAsForward(t *testing.T) {
	// .cmd command strings are authored relative to the fighter's own
	// facing (F = forward, whichever way that currently is) -- a fighter
	// facing left must read a raw Left press as "forward", not "back".
	cmds := qcfCommands()
	s := State{}

	ticks := []TickInput{
		{Down: true},
		{Down: true, Left: true},
		{Left: true},
		{Buttons: map[string]bool{"a": true}},
	}

	var active map[string]bool
	for i, in := range ticks {
		s, active = Step(s, i, match.FacingLeft, in, cmds)
	}

	if !active["QCF_a"] {
		t.Errorf("QCF_a not recognized from a facing-left fighter's raw-left quarter circle; active = %+v", active)
	}
}

func TestStep_RecognizedCommandStaysActiveOnlyForItsBufferWindow(t *testing.T) {
	cmds := qcfCommands() // Defaults.BufferTime = 1
	s := State{}

	ticks := []TickInput{
		{Down: true},
		{Down: true, Right: true},
		{Right: true},
		{Buttons: map[string]bool{"a": true}},
	}
	var active map[string]bool
	for i, in := range ticks {
		s, active = Step(s, i, match.FacingRight, in, cmds)
	}
	if !active["QCF_a"] {
		t.Fatalf("QCF_a not recognized at the completing tick; active = %+v", active)
	}

	// BufferTime = 1: it should still read active exactly one tick later...
	s, active = Step(s, len(ticks), match.FacingRight, TickInput{}, cmds)
	if !active["QCF_a"] {
		t.Errorf("QCF_a dropped before its 1-tick buffer window elapsed; active = %+v", active)
	}

	// ...but not two ticks later.
	_, active = Step(s, len(ticks)+1, match.FacingRight, TickInput{}, cmds)
	if active["QCF_a"] {
		t.Errorf("QCF_a still active after its 1-tick buffer window elapsed; active = %+v", active)
	}
}

func TestStep_MultipleCommands_AreTrackedIndependently(t *testing.T) {
	cmds := cmd.CommandFile{
		Defaults: cmd.CommandDefaults{Time: 15, BufferTime: 1},
		Commands: []cmd.Command{
			{Name: "holdback", Input: "B"},
			{Name: "holdfwd", Input: "F"},
		},
	}
	s := State{}

	_, active := Step(s, 0, match.FacingRight, TickInput{Left: true}, cmds)

	if !active["holdback"] {
		t.Errorf("holdback not recognized while holding back; active = %+v", active)
	}
	if active["holdfwd"] {
		t.Errorf("holdfwd incorrectly recognized while holding back, not forward; active = %+v", active)
	}
}

// kfmDashCommands are kfm.cmd's own "FF" (F, F within 10 ticks) and
// "holdfwd" (/$F) commands (testdata/reference/chars/kfm/kfm.cmd:248-251,
// 312-314).
func kfmDashCommands() cmd.CommandFile {
	return cmd.CommandFile{
		Defaults: cmd.CommandDefaults{Time: 15, BufferTime: 1},
		Commands: []cmd.Command{
			{Name: "FF", Input: "F, F", Time: 10},
			{Name: "holdfwd", Input: "/$F", Time: 1},
		},
	}
}

func TestStep_ForwardHeldWithoutRelease_IsOnePressNotADoubleTap(t *testing.T) {
	// Ikemen GO input.go:2621-2625: a step without "/" matches only on the
	// tick its key becomes pressed (State == 1); "/" matches while held.
	cmds := kfmDashCommands()
	s := State{}
	var active map[string]bool
	for tick := 0; tick < 6; tick++ {
		s, active = Step(s, tick, match.FacingRight, TickInput{Right: true}, cmds)
		if active["FF"] {
			t.Fatalf("tick %d: FF recognized while forward is only held", tick)
		}
		if !active["holdfwd"] {
			t.Fatalf("tick %d: holdfwd not recognized while forward is held", tick)
		}
	}
}

func TestStep_ForwardTappedTwice_IsADoubleTap(t *testing.T) {
	cmds := kfmDashCommands()
	s := State{}
	var active map[string]bool
	for tick, in := range []TickInput{{Right: true}, {}, {Right: true}} {
		s, active = Step(s, tick, match.FacingRight, in, cmds)
	}
	if !active["FF"] {
		t.Errorf("FF not recognized after forward, neutral, forward; active = %+v", active)
	}
}

// Two [Command] blocks named alike are alternatives (Ikemen GO keeps one
// Command per block): each has its own progress, and either activates the
// name. A shorter one used to read the longer one's step and panic (backlog
// item 038; Arale, Necro, Fiona Belli).
func TestStep_SameNameAlternativesKeepTheirOwnProgress(t *testing.T) {
	cmds := cmd.CommandFile{Defaults: cmd.CommandDefaults{Time: 15, BufferTime: 1}, Commands: []cmd.Command{
		{Name: "fireball", Input: "~D, F, x"},
		{Name: "fireball", Input: "~D, DF, F, x"},
	}}
	var st State
	feed := func(seq []TickInput) bool {
		active := false
		for i, in := range seq {
			var a map[string]bool
			st, a = Step(st, i+1, match.FacingRight, in, cmds)
			active = active || a["fireball"]
		}
		return active
	}
	down, df, fwd := TickInput{Down: true}, TickInput{Down: true, Right: true}, TickInput{Right: true}
	x := TickInput{Right: true, Buttons: map[string]bool{"x": true}}
	if !feed([]TickInput{{}, down, df, fwd, x}) {
		t.Error("the long alternative did not activate the name")
	}
	st = State{}
	if !feed([]TickInput{{}, down, {}, fwd, x}) {
		t.Error("the short alternative did not activate the name")
	}
}

// input.go:901-960: "$F" holds on F, UF and DF; plain "F" only on F.
func TestStep_DollarDirectionsIgnoreTheOtherDirections(t *testing.T) {
	cf := cmd.CommandFile{Commands: []cmd.Command{
		{Name: "holdfwd", Input: "/$F", Time: 1},
		{Name: "plainfwd", Input: "/F", Time: 1},
		{Name: "holddown", Input: "/$D", Time: 1},
	}}
	cases := []struct {
		in   TickInput
		want map[string]bool
	}{
		{TickInput{Right: true}, map[string]bool{"holdfwd": true, "plainfwd": true}},
		{TickInput{Up: true, Right: true}, map[string]bool{"holdfwd": true}},
		{TickInput{Down: true, Left: true}, map[string]bool{"holddown": true}},
	}
	for _, c := range cases {
		var st State
		var act map[string]bool
		for tick := 1; tick <= 2; tick++ {
			st, act = Step(st, tick, match.FacingRight, c.in, cf)
		}
		for _, name := range []string{"holdfwd", "plainfwd", "holddown"} {
			if act[name] != c.want[name] {
				t.Errorf("%+v: %s = %v, want %v", c.in, name, act[name], c.want[name])
			}
		}
	}
}
