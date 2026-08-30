package round

import (
	"testing"

	"github.com/openkakutou/engine/match"
)

func newState(t *testing.T, round, roundTimer, p1Health, p2Health int) *match.MatchState {
	t.Helper()
	ms, err := match.NewMatchState(
		round, roundTimer,
		match.FighterState{Side: match.SideP1, Health: p1Health},
		match.FighterState{Side: match.SideP2, Health: p2Health},
	)
	if err != nil {
		t.Fatalf("failed to build match state: %v", err)
	}
	return ms
}

func TestCheckOutcome_ReportsKO_WhenP1HealthReachesZero(t *testing.T) {
	state := newState(t, 1, 100, 0, 500)

	result := CheckOutcome(state)

	if result.Outcome != OutcomeKO {
		t.Fatalf("Outcome = %v, want OutcomeKO", result.Outcome)
	}
	if result.Winner != match.SideP2 {
		t.Errorf("Winner = %v, want SideP2", result.Winner)
	}
}

func TestCheckOutcome_ReportsKO_WhenP2HealthReachesZero(t *testing.T) {
	state := newState(t, 1, 100, 500, 0)

	result := CheckOutcome(state)

	if result.Outcome != OutcomeKO {
		t.Fatalf("Outcome = %v, want OutcomeKO", result.Outcome)
	}
	if result.Winner != match.SideP1 {
		t.Errorf("Winner = %v, want SideP1", result.Winner)
	}
}

func TestCheckOutcome_ReportsDoubleKO_WhenBothFightersReachZeroHealthSameTick(t *testing.T) {
	state := newState(t, 1, 100, 0, 0)

	result := CheckOutcome(state)

	if result.Outcome != OutcomeDoubleKO {
		t.Fatalf("Outcome = %v, want OutcomeDoubleKO", result.Outcome)
	}
}

func TestCheckOutcome_ReportsTimeout_WithHealthTiebreak(t *testing.T) {
	state := newState(t, 1, 0, 300, 200)

	result := CheckOutcome(state)

	if result.Outcome != OutcomeTimeout {
		t.Fatalf("Outcome = %v, want OutcomeTimeout", result.Outcome)
	}
	if result.Winner != match.SideP1 {
		t.Errorf("Winner = %v, want SideP1 (more health)", result.Winner)
	}
}

func TestCheckOutcome_ReportsTimeoutDraw_WhenHealthIsEqual(t *testing.T) {
	state := newState(t, 1, 0, 250, 250)

	result := CheckOutcome(state)

	if result.Outcome != OutcomeTimeoutDraw {
		t.Fatalf("Outcome = %v, want OutcomeTimeoutDraw", result.Outcome)
	}
}

func TestCheckOutcome_ReportsKO_NotTimeout_WhenBothConditionsHoldSameTick(t *testing.T) {
	// A fighter that runs out of health on the exact tick the round timer
	// also expires must still be reported as a KO, matching real MUGEN's
	// own precedence -- checked before the caller (round-level logic)
	// would otherwise be tempted to treat this as a health-tiebreak
	// timeout instead.
	state := newState(t, 1, 0, 0, 500)

	result := CheckOutcome(state)

	if result.Outcome != OutcomeKO {
		t.Fatalf("Outcome = %v, want OutcomeKO (KO takes precedence over timeout)", result.Outcome)
	}
}

func TestCheckOutcome_ReportsOutcomeNone_WhileRoundStillInProgress(t *testing.T) {
	state := newState(t, 1, 100, 500, 500)

	result := CheckOutcome(state)

	if result.Outcome != OutcomeNone {
		t.Fatalf("Outcome = %v, want OutcomeNone", result.Outcome)
	}
}

func TestResetRound_RestoresBothFightersToTheirStartingStateAndAdvancesRound(t *testing.T) {
	p1Start := match.FighterState{Side: match.SideP1, Health: 1000, Position: match.Position{X: -50}}
	p2Start := match.FighterState{Side: match.SideP2, Health: 1000, Position: match.Position{X: 50}}

	state, err := ResetRound(2, 3000, p1Start, p2Start)
	if err != nil {
		t.Fatalf("ResetRound returned an error: %v", err)
	}

	if state.Round != 2 {
		t.Errorf("Round = %d, want 2", state.Round)
	}
	if state.RoundTimer != 3000 {
		t.Errorf("RoundTimer = %d, want 3000", state.RoundTimer)
	}
	if got := state.Fighter(match.SideP1); got != p1Start {
		t.Errorf("P1 fighter state = %+v, want %+v", got, p1Start)
	}
	if got := state.Fighter(match.SideP2); got != p2Start {
		t.Errorf("P2 fighter state = %+v, want %+v", got, p2Start)
	}
}

func TestResetRound_ReturnsError_OnMalformedStartingFighters(t *testing.T) {
	// Both fighters declaring the same Side is exactly the malformed input
	// match.NewMatchState already rejects -- ResetRound must surface that,
	// not panic or silently build a broken MatchState.
	p1Start := match.FighterState{Side: match.SideP1, Health: 1000}
	duplicateSide := match.FighterState{Side: match.SideP1, Health: 1000}

	_, err := ResetRound(2, 3000, p1Start, duplicateSide)
	if err == nil {
		t.Fatal("expected an error for two fighters on the same Side, got nil")
	}
}

func TestNewProgress_ReturnsError_OnEvenBestOf(t *testing.T) {
	_, err := NewProgress(4)
	if err == nil {
		t.Fatal("expected an error for an even bestOf, got nil")
	}
}

func TestNewProgress_ReturnsError_OnNonPositiveBestOf(t *testing.T) {
	_, err := NewProgress(0)
	if err == nil {
		t.Fatal("expected an error for a non-positive bestOf, got nil")
	}
}

func TestProgress_RoundsToWin_IsHalfBestOfPlusOne(t *testing.T) {
	p, err := NewProgress(3)
	if err != nil {
		t.Fatalf("NewProgress returned an error: %v", err)
	}
	if got := p.RoundsToWin(); got != 2 {
		t.Errorf("RoundsToWin() = %d, want 2", got)
	}
}

func TestProgress_RecordRoundResult_TracksWinsPerSideAcrossBestOfN(t *testing.T) {
	p, err := NewProgress(3)
	if err != nil {
		t.Fatalf("NewProgress returned an error: %v", err)
	}

	p = p.RecordRoundResult(RoundResult{Outcome: OutcomeKO, Winner: match.SideP1})
	p = p.RecordRoundResult(RoundResult{Outcome: OutcomeTimeout, Winner: match.SideP2})

	if p.Wins[match.SideP1] != 1 {
		t.Errorf("Wins[SideP1] = %d, want 1", p.Wins[match.SideP1])
	}
	if p.Wins[match.SideP2] != 1 {
		t.Errorf("Wins[SideP2] = %d, want 1", p.Wins[match.SideP2])
	}
	if p.RoundsPlayed != 2 {
		t.Errorf("RoundsPlayed = %d, want 2", p.RoundsPlayed)
	}
}

func TestProgress_RecordRoundResult_ADrawAwardsNoRoundWinToEitherSide(t *testing.T) {
	p, err := NewProgress(3)
	if err != nil {
		t.Fatalf("NewProgress returned an error: %v", err)
	}

	p = p.RecordRoundResult(RoundResult{Outcome: OutcomeDoubleKO})

	if p.Wins[match.SideP1] != 0 || p.Wins[match.SideP2] != 0 {
		t.Errorf("Wins = %v, want both 0 after a draw", p.Wins)
	}
	if p.RoundsPlayed != 1 {
		t.Errorf("RoundsPlayed = %d, want 1", p.RoundsPlayed)
	}
}

func TestProgress_MatchOutcome_DecidedOnceASideReachesRoundsToWin(t *testing.T) {
	p, err := NewProgress(3)
	if err != nil {
		t.Fatalf("NewProgress returned an error: %v", err)
	}

	p = p.RecordRoundResult(RoundResult{Outcome: OutcomeKO, Winner: match.SideP2})
	if decided, _ := p.MatchOutcome(); decided {
		t.Fatal("match reported decided after only 1 of 2 needed round wins")
	}

	p = p.RecordRoundResult(RoundResult{Outcome: OutcomeKO, Winner: match.SideP2})
	decided, winner := p.MatchOutcome()
	if !decided {
		t.Fatal("expected the match to be decided after 2 round wins (bestOf 3)")
	}
	if winner != match.SideP2 {
		t.Errorf("winner = %v, want SideP2", winner)
	}
}

func TestProgress_MatchOutcome_NotDecided_OnFreshProgress(t *testing.T) {
	p, err := NewProgress(3)
	if err != nil {
		t.Fatalf("NewProgress returned an error: %v", err)
	}

	if decided, _ := p.MatchOutcome(); decided {
		t.Fatal("a fresh Progress must not report the match as decided")
	}
}
