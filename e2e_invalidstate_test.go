package engine

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// End-to-end scenario for backlog 068: Kung Fu Man loaded WITHOUT common1
// (no search folder, so every common state is missing) throws a light punch.
// State 200 ends with a ChangeState to state 0, which the character does not
// have. Ikemen GO warns "changed to invalid state 0 (from state 200)" and
// enters an empty state whose state type, move type and physics are
// unchanged (char.go:6880-6887); the match goes on.

// loadWithoutCommon loads a reference character with no search folder.
func loadWithoutCommon(t *testing.T, defPath string) FighterProgram {
	t.Helper()
	path := filepath.Join(referenceDir, defPath)
	prog, missing, err := LoadFighterProgram(loadChar(t, path), path)
	if err != nil {
		t.Fatalf("LoadFighterProgram(%s): %v", defPath, err)
	}
	if len(missing) == 0 {
		t.Fatalf("LoadFighterProgram(%s): want the common states file missing", defPath)
	}
	return prog
}

func scenarioChangeToMissingState(t *testing.T, prog FighterProgram) {
	t.Helper()
	if prog.hasState(0) {
		t.Fatal("fixture error: the program must not have state 0")
	}
	m := newE2EMatchAt(t, prog, 200)

	var warnings []string
	for i := 1; i <= 80 && len(warnings) == 0; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			t.Fatalf("a change to a missing state must not stop the match: %v", err)
		}
		warnings = slices.Clone(m.warnings[match.SideP1])
	}
	want := []string{"changed to invalid state 0 (from state 200)"}
	if !slices.Equal(warnings, want) {
		t.Fatalf("warnings = %q, want %q", warnings, want)
	}
	if got := m.state.Fighters[match.SideP1].StateNo; got != 0 {
		t.Fatalf("StateNo = %d, want 0 (the empty state, numbered as requested)", got)
	}
	ctx := m.runtimes[match.SideP1].Context
	if ctx.PrevStateNo != 200 {
		t.Errorf("PrevStateNo = %d, want 200", ctx.PrevStateNo)
	}
	// State 200 declares type S, movetype A, physics S: all stay (an empty
	// Ikemen state defaults would be movetype I, physics N).
	if ctx.StateType != 'S' || ctx.MoveType != 'A' || ctx.Physics != 'S' {
		t.Errorf("type, movetype, physics = %c %c %c, want S A S (unchanged)", ctx.StateType, ctx.MoveType, ctx.Physics)
	}
	// The change itself wrote ctrl = 1 (kfm.cns:303); the empty state leaves it.
	if !ctx.Ctrl {
		t.Error("Ctrl = false, want true as the ChangeState set it")
	}
	for i := 0; i < 10; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			t.Fatalf("the match must keep ticking in the empty state: %v", err)
		}
		if len(m.warnings[match.SideP1]) != 0 {
			t.Errorf("warnings = %q on a tick with no state change, want none", m.warnings[match.SideP1])
		}
	}
	if got := m.state.Fighters[match.SideP1].StateNo; got != 0 {
		t.Errorf("StateNo = %d after ticking in the empty state, want 0", got)
	}
	// Time restarts at the change and grows at the end of that tick (1), then
	// once per tick (10 more): the empty state keeps counting.
	if m.runtimes[match.SideP1].Context.Time != 11 {
		t.Errorf("Time = %d, want 11", m.runtimes[match.SideP1].Context.Time)
	}
}

func TestE2E_KfmCns_ChangeToMissingState_EntersEmptyStateAndKeepsTicking(t *testing.T) {
	scenarioChangeToMissingState(t, loadWithoutCommon(t, kfmCnsDef))
}

func TestE2E_KfmZss_ChangeToMissingState_EntersEmptyStateAndKeepsTicking(t *testing.T) {
	scenarioChangeToMissingState(t, loadWithoutCommon(t, kfmZssDef))
}
