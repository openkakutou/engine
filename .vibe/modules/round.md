# Module: round

**Role:** Round/match flow on top of `match.MatchState`: deciding how a round ends (KO, double KO, timeout with a health tiebreak), resetting fighters to a fresh starting state for the next round, and tracking round wins per side across a best-of-N match. Match-level bookkeeping is a wrapper type here, not added to the already-closed `match.MatchState`.
**Files:** `round/round.go`
**Exports:** `Outcome` (int enum: `OutcomeNone`, `OutcomeKO`, `OutcomeDoubleKO`, `OutcomeTimeout`, `OutcomeTimeoutDraw`); `RoundResult`; `CheckOutcome(state *match.MatchState) RoundResult`; `ResetRound(nextRound, roundTimer int, p1Start, p2Start match.FighterState) (*match.MatchState, error)`; `Progress`; `NewProgress(bestOf int) (Progress, error)`; `Progress.RoundsToWin() int`; `Progress.RecordRoundResult(result RoundResult) Progress`; `Progress.MatchOutcome() (decided bool, winner match.Side)`
**Depends on:** `modules/match.md`
