// Package round implements round/match flow on top of match.MatchState:
// deciding how a round ends (KO, timeout with a health tiebreak, or a
// double KO), resetting fighters to a fresh starting state for the next
// round, and tracking which side has won how many rounds across a
// best-of-N match. This is the last piece tying engine's per-tick
// packages into a usable match loop -- see the root engine package's Tick
// for how CheckOutcome fits into a full simulation tick, and cmd/wasm for
// how a mode-* game app drives all of it.
//
// Match-level bookkeeping (rounds won per side, best-of-N) is not carried
// on match.MatchState itself -- that item is already closed, and this
// package follows the same wrap-rather-than-reopen precedent
// evaluator.Context and physics's derived "grounded" state already set
// (see .vibe/decisions/002 and physics's own .vibe/decisions/007):
// Progress is its own type here instead.
package round

import (
	"fmt"

	"github.com/openkakutou/engine/match"
)

// Outcome describes how a round ended, or that it hasn't yet.
type Outcome int

const (
	// OutcomeNone means the round is still in progress -- neither fighter
	// has been KO'd and the round timer has not expired.
	OutcomeNone Outcome = iota
	// OutcomeKO means one fighter's health reached zero this tick.
	OutcomeKO
	// OutcomeDoubleKO means both fighters' health reached zero on the same
	// tick -- a degenerate but real MUGEN/Ikemen scenario, resolved here
	// as a round with no winner rather than a crash or an arbitrarily
	// picked side.
	OutcomeDoubleKO
	// OutcomeTimeout means the round timer expired with the fighters at
	// unequal health -- the fighter with more health remaining wins.
	OutcomeTimeout
	// OutcomeTimeoutDraw means the round timer expired with both fighters
	// at exactly equal health -- neither wins this round.
	OutcomeTimeoutDraw
)

// RoundResult reports how a round ended. Winner is only meaningful for
// OutcomeKO and OutcomeTimeout -- a zero RoundResult (Outcome: OutcomeNone)
// means the round is still in progress.
type RoundResult struct {
	Outcome Outcome    `json:"outcome"`
	Winner  match.Side `json:"winner"`
}

// CheckOutcome inspects state for a round-ending condition as of this
// tick: a KO when either or both fighters' health has reached zero, or a
// timeout when RoundTimer has reached zero, broken by comparing remaining
// health. A KO is checked before a timeout, so a fighter that runs out of
// health on the exact tick the round timer also expires is still reported
// as a KO, matching real MUGEN/Ikemen's own precedence. Returns a zero
// RoundResult when neither condition holds -- the round continues.
func CheckOutcome(state *match.MatchState) RoundResult {
	p1 := state.Fighter(match.SideP1)
	p2 := state.Fighter(match.SideP2)
	p1Dead := p1.Health <= 0
	p2Dead := p2.Health <= 0

	switch {
	case p1Dead && p2Dead:
		return RoundResult{Outcome: OutcomeDoubleKO}
	case p1Dead:
		return RoundResult{Outcome: OutcomeKO, Winner: match.SideP2}
	case p2Dead:
		return RoundResult{Outcome: OutcomeKO, Winner: match.SideP1}
	}

	if state.RoundTimer <= 0 {
		switch {
		case p1.Health > p2.Health:
			return RoundResult{Outcome: OutcomeTimeout, Winner: match.SideP1}
		case p2.Health > p1.Health:
			return RoundResult{Outcome: OutcomeTimeout, Winner: match.SideP2}
		default:
			return RoundResult{Outcome: OutcomeTimeoutDraw}
		}
	}

	return RoundResult{}
}

// ResetRound builds a fresh MatchState for the next round: both fighters
// restored to their given starting state/position/health -- supplied by
// the caller, since engine holds no character-loadout data of its own --
// with the round number and timer set for the round about to begin. It is
// a thin, documented wrapper over match.NewMatchState, surfacing that
// constructor's own validation (e.g. two starting fighters declaring the
// same Side) as an error rather than a panic.
func ResetRound(nextRound, roundTimer int, p1Start, p2Start match.FighterState) (*match.MatchState, error) {
	state, err := match.NewMatchState(nextRound, roundTimer, p1Start, p2Start)
	if err != nil {
		return nil, fmt.Errorf("round: ResetRound: %w", err)
	}
	return state, nil
}

// Progress tracks match-level bookkeeping across rounds -- how many
// rounds each side has won, out of a best-of-N match -- that
// match.MatchState itself does not carry (see the package doc comment).
// Its zero value is not usable directly; build one with NewProgress.
type Progress struct {
	// BestOf is the total number of rounds the match is played to decide
	// a winner; always a positive odd number, so a match can never end in
	// a tie at the round level.
	BestOf int `json:"bestOf"`
	// Wins is the number of rounds each side has won so far, indexed by
	// match.Side.
	Wins [2]int `json:"wins"`
	// RoundsPlayed is the total number of rounds recorded so far,
	// including drawn rounds that awarded neither side a win.
	RoundsPlayed int `json:"roundsPlayed"`
}

// NewProgress builds a fresh Progress for a match played to bestOf rounds.
// It returns a descriptive error, rather than a Progress that could never
// be won outright, if bestOf is not a positive odd number.
func NewProgress(bestOf int) (Progress, error) {
	if bestOf < 1 || bestOf%2 == 0 {
		return Progress{}, fmt.Errorf("round: NewProgress requires a positive odd bestOf, got %d", bestOf)
	}
	return Progress{BestOf: bestOf}, nil
}

// RoundsToWin is the number of round wins either side needs to win the
// match outright.
func (p Progress) RoundsToWin() int {
	return p.BestOf/2 + 1
}

// RecordRoundResult returns p advanced by one more played round: result's
// winner (if any) gains one round win, and RoundsPlayed always advances by
// one. A drawn round (OutcomeDoubleKO or OutcomeTimeoutDraw) awards no
// round win to either side, matching real MUGEN/Ikemen behavior -- p
// itself is never mutated.
func (p Progress) RecordRoundResult(result RoundResult) Progress {
	out := p
	out.RoundsPlayed++
	switch result.Outcome {
	case OutcomeKO, OutcomeTimeout:
		out.Wins[result.Winner]++
	}
	return out
}

// MatchOutcome reports whether the match is decided yet: decided is true
// once either side has reached RoundsToWin round wins, and winner is only
// meaningful when decided is true.
func (p Progress) MatchOutcome() (decided bool, winner match.Side) {
	toWin := p.RoundsToWin()
	if p.Wins[match.SideP1] >= toWin {
		return true, match.SideP1
	}
	if p.Wins[match.SideP2] >= toWin {
		return true, match.SideP2
	}
	return false, 0
}
