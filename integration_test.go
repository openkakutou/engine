package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cmd"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/engine/round"
	"github.com/openkakutou/stage"
)

// loadFighterProgram parses this package's own testdata/fighter.{cns,air,
// cmd} -- a small but real state loop (idle, an attack with a HitDef, and
// the return to idle), styled after real MUGEN/Ikemen idioms the same way
// this repo's own kfm_idle.cns/kfm_idle.zss testdata already are, not a
// literal downloaded character. See testdata/fighter.cns's own header
// comment.
func loadFighterProgram(t *testing.T) FighterProgram {
	t.Helper()

	cnsFile, err := os.Open(filepath.Join("testdata", "fighter.cns"))
	if err != nil {
		t.Fatalf("failed to open fighter.cns: %v", err)
	}
	defer cnsFile.Close()
	states, err := cns.Parse(cnsFile)
	if err != nil {
		t.Fatalf("failed to parse fighter.cns: %v", err)
	}
	statesByNumber := make(map[int]cns.StateDef, len(states))
	for _, s := range states {
		statesByNumber[s.Number] = s
	}

	airFile, err := os.Open(filepath.Join("testdata", "fighter.air"))
	if err != nil {
		t.Fatalf("failed to open fighter.air: %v", err)
	}
	defer airFile.Close()
	anims, err := air.Parse(airFile)
	if err != nil {
		t.Fatalf("failed to parse fighter.air: %v", err)
	}

	cmdFile, err := os.Open(filepath.Join("testdata", "fighter.cmd"))
	if err != nil {
		t.Fatalf("failed to open fighter.cmd: %v", err)
	}
	defer cmdFile.Close()
	commands, err := cmd.Parse(cmdFile)
	if err != nil {
		t.Fatalf("failed to parse fighter.cmd: %v", err)
	}

	return FighterProgram{States: statesByNumber, Animations: anims, Commands: commands}
}

// loadStageBounds parses this package's own testdata/stage.def -- copied
// verbatim from the sibling stage repo's own cmd/wasm/testdata/sample.def,
// the same "copy the real fixture, don't reach across repos at test time"
// convention hitdetect/testdata/sample.air already established (copied
// from character/air/testdata/sample.air).
func loadStageBounds(t *testing.T) *stage.StageBoundaries {
	t.Helper()

	f, err := os.Open(filepath.Join("testdata", "stage.def"))
	if err != nil {
		t.Fatalf("failed to open stage.def: %v", err)
	}
	defer f.Close()
	s, err := stage.Parse(f)
	if err != nil {
		t.Fatalf("failed to parse stage.def: %v", err)
	}
	return &s.StageBoundaries
}

// TestIntegration_FullScriptedMatch_TwoRealCharactersRunsToACorrectWinCondition
// is backlog item 009's own required fixture-driven integration test: a
// full scripted match between two real characters (loaded from real,
// parsed .cns/.air/.cmd data, and real stage boundary data), driven
// end-to-end through Tick and round alone, asserting on the final match
// outcome -- not just individual subsystem behavior. P1 holds its attack
// command every tick; P2 never presses anything -- a deterministic,
// one-sided script exercising the state transition -> HitDef -> damage ->
// KO -> round reset -> best-of-N progression loop exactly the way a mode-*
// game app's own per-frame loop would drive it via the WASM entrypoint.
func TestIntegration_FullScriptedMatch_TwoRealCharactersRunsToACorrectWinCondition(t *testing.T) {
	prog := loadFighterProgram(t)
	bounds := loadStageBounds(t)

	const startingHealth = 60 // 4 landed hits (15 damage each) to KO
	const roundTimer = 10000  // generous -- this scripted match always ends in a KO, never a timeout
	const bestOf = 3

	p1Start := match.FighterState{Side: match.SideP1, Facing: match.FacingRight, Health: startingHealth}
	p2Start := match.FighterState{Side: match.SideP2, Facing: match.FacingLeft, Health: startingHealth}

	progress, err := round.NewProgress(bestOf)
	if err != nil {
		t.Fatalf("round.NewProgress: %v", err)
	}

	state, err := match.NewMatchState(1, roundTimer, p1Start, p2Start)
	if err != nil {
		t.Fatalf("match.NewMatchState: %v", err)
	}

	p1Runtime, err := NewFighterRuntime(p1Start, prog.States)
	if err != nil {
		t.Fatalf("NewFighterRuntime(p1): %v", err)
	}
	p2Runtime, err := NewFighterRuntime(p2Start, prog.States)
	if err != nil {
		t.Fatalf("NewFighterRuntime(p2): %v", err)
	}

	programs := [2]FighterProgram{match.SideP1: prog, match.SideP2: prog}
	runtimes := [2]FighterRuntime{match.SideP1: p1Runtime, match.SideP2: p2Runtime}

	p1Input := input.TickInput{Buttons: map[string]bool{"a": true}}
	var p2Input input.TickInput // never attacks

	const maxTicks = 5000
	var decided bool
	var winner match.Side

	for tick := 1; tick <= maxTicks; tick++ {
		result, err := Tick(*state, programs, runtimes, [2]input.TickInput{match.SideP1: p1Input, match.SideP2: p2Input}, TickConfig{Bounds: bounds, Gravity: 0, Tick: tick, ComboWindow: 60})
		if err != nil {
			t.Fatalf("Tick failed at tick %d: %v", tick, err)
		}
		*state = result.State
		runtimes = result.Fighters

		if result.Round.Outcome == round.OutcomeNone {
			continue
		}

		progress = progress.RecordRoundResult(result.Round)
		if d, w := progress.MatchOutcome(); d {
			decided, winner = d, w
			break
		}

		nextState, err := round.ResetRound(state.Round+1, roundTimer, p1Start, p2Start)
		if err != nil {
			t.Fatalf("round.ResetRound: %v", err)
		}
		*state = *nextState

		p1Runtime, err = NewFighterRuntime(p1Start, prog.States)
		if err != nil {
			t.Fatalf("NewFighterRuntime(p1) after round reset: %v", err)
		}
		p2Runtime, err = NewFighterRuntime(p2Start, prog.States)
		if err != nil {
			t.Fatalf("NewFighterRuntime(p2) after round reset: %v", err)
		}
		runtimes = [2]FighterRuntime{match.SideP1: p1Runtime, match.SideP2: p2Runtime}
	}

	if !decided {
		t.Fatalf("match did not reach a decided outcome within %d ticks", maxTicks)
	}
	if winner != match.SideP1 {
		t.Errorf("winner = %v, want SideP1 (P2 never attacks, so P1 must win every round)", winner)
	}
	if want := progress.RoundsToWin(); progress.RoundsPlayed != want {
		t.Errorf("RoundsPlayed = %d, want %d (P1 wins every round straight, no draws or extra rounds)", progress.RoundsPlayed, want)
	}
	if got := progress.Wins[match.SideP2]; got != 0 {
		t.Errorf("P2's round wins = %d, want 0", got)
	}
}
