package input

import "testing"

func TestParseSteps_QuarterCircleForwardPunch_ReturnsFourSteps(t *testing.T) {
	// The exact real-world command this item's own "Done" criterion names:
	// character/cmd's sample.cmd fixture defines "QCF_a" as this string.
	got := parseSteps(`~D, DF, F, a`)

	if len(got) != 4 {
		t.Fatalf("parseSteps returned %d steps, want 4: %+v", len(got), got)
	}

	want := []step{
		{hasDir: true, dirs: directionSet{down: true}},
		{hasDir: true, dirs: directionSet{down: true, forward: true}},
		{hasDir: true, dirs: directionSet{forward: true}},
		{buttons: []string{"a"}},
	}
	for i, w := range want {
		if !stepEqual(got[i], w) {
			t.Errorf("step %d = %+v, want %+v", i, got[i], w)
		}
	}
}

func TestParseSteps_SimultaneousDirectionAndButton_CombinesBothInOneStep(t *testing.T) {
	got := parseSteps(`D+a`)

	if len(got) != 1 {
		t.Fatalf("parseSteps returned %d steps, want 1: %+v", len(got), got)
	}
	want := step{hasDir: true, dirs: directionSet{down: true}, buttons: []string{"a"}}
	if !stepEqual(got[0], want) {
		t.Errorf("step 0 = %+v, want %+v", got[0], want)
	}
}

func TestParseSteps_MultipleSimultaneousButtons_RequiresAllOfThem(t *testing.T) {
	got := parseSteps(`b+y`)

	if len(got) != 1 {
		t.Fatalf("parseSteps returned %d steps, want 1: %+v", len(got), got)
	}
	want := step{buttons: []string{"b", "y"}}
	if !stepEqual(got[0], want) {
		t.Errorf("step 0 = %+v, want %+v", got[0], want)
	}
}

func TestParseSteps_EmptyInput_ReturnsNoSteps(t *testing.T) {
	got := parseSteps(``)
	if len(got) != 0 {
		t.Errorf("parseSteps(\"\") returned %d steps, want 0: %+v", len(got), got)
	}
}

func TestParseSteps_StraySeparatorProducesNoEmptyStep(t *testing.T) {
	// A stray double comma (e.g. a trailing one, or an authoring slip) must
	// not produce a bogus empty step that could never be matched, which
	// would permanently block the whole command from ever completing.
	got := parseSteps(`D,,F`)

	if len(got) != 2 {
		t.Fatalf("parseSteps returned %d steps, want 2: %+v", len(got), got)
	}
}

// stepEqual compares two step values for the purpose of these tests --
// slice equality (buttons) needs element-wise comparison, so a plain ==
// isn't available on step.
func stepEqual(a, b step) bool {
	if a.hasDir != b.hasDir || a.dirs != b.dirs {
		return false
	}
	if len(a.buttons) != len(b.buttons) {
		return false
	}
	for i := range a.buttons {
		if a.buttons[i] != b.buttons[i] {
			return false
		}
	}
	return true
}
