package engine

import (
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/match"
	"github.com/openkakutou/engine/v2/round"
)

// roundStartState is the state a fighter enters at a round's start
// (system.go:2595): common1's initialisation, which goes on to the intro.
const roundStartState = 5900

// setRoundProgress gives a fighter the round's state for this tick: 2 (the
// fight) without one.
func setRoundProgress(ctx *evaluator.Context, p *RoundProgress) {
	if p == nil {
		ctx.RoundState, ctx.RoundEnd = 2, evaluator.RoundEnd{}
		return
	}
	ctx.RoundState = p.State
	ctx.RoundEnd = evaluator.RoundEnd{MatchOver: p.MatchOver}
	switch p.Result.Outcome {
	case round.OutcomeKO:
		ctx.RoundEnd = evaluator.RoundEnd{Decided: true, Winner: p.Result.Winner, KO: true, MatchOver: p.MatchOver}
	case round.OutcomeTimeout:
		ctx.RoundEnd = evaluator.RoundEnd{Decided: true, Winner: p.Result.Winner, MatchOver: p.MatchOver}
	case round.OutcomeDoubleKO:
		ctx.RoundEnd = evaluator.RoundEnd{Decided: true, Draw: true, KO: true, MatchOver: p.MatchOver}
	case round.OutcomeTimeoutDraw:
		ctx.RoundEnd = evaluator.RoundEnd{Decided: true, Draw: true, MatchOver: p.MatchOver}
	}
}

// NewRoundRuntime is a fighter's runtime at a round's start, as Ikemen GO
// starts it (system.go:2580-2595): in state 5900 with animation 0 and no
// control, when its character has that state; else as NewRuntime. Its
// Context.FighterState is the fighter's match state for the round.
func (p FighterProgram) NewRoundRuntime(fighter match.FighterState) (FighterRuntime, error) {
	if !p.hasState(roundStartState) {
		return p.NewRuntime(fighter)
	}
	fighter.StateNo = roundStartState
	rt, err := p.NewRuntime(fighter)
	if err != nil {
		return rt, err
	}
	rt.Context.Ctrl = false
	return rt, nil
}

// StartFight is the fight's start (system.go:3353-3368): a living fighter
// gets control and, unless it asserts nointroreset, goes back to state 0. It
// changes the runtimes and the match state alike.
func StartFight(programs [2]FighterProgram, state *match.MatchState, runtimes *[2]FighterRuntime) error {
	for i := range runtimes {
		rt := &runtimes[i]
		if rt.Context.Health <= 0 {
			continue
		}
		rt.Context.Ctrl = true
		if rt.Context.StateNo != 0 && !rt.Context.Asserted.Has("nointroreset") {
			if err := programs[i].selfState(rt, 0); err != nil {
				return err
			}
		}
		state.Fighters[i] = rt.Context.FighterState
	}
	return nil
}

// EnterPoses sends the fighters to their round-end poses
// (system.go:3510-3536): a living fighter not in a hit pause enters 180 when
// it won, 170 when it lost, 175 on a draw. It changes the runtimes and the
// match state alike.
func EnterPoses(programs [2]FighterProgram, state *match.MatchState, runtimes *[2]FighterRuntime, result round.RoundResult) error {
	for i := range runtimes {
		rt := &runtimes[i]
		if rt.Context.Health <= 0 || rt.HitPause > 0 || rt.Context.GetHit.HitShakeTime > 0 {
			continue
		}
		pose := 175
		if result.Outcome == round.OutcomeKO || result.Outcome == round.OutcomeTimeout {
			pose = 170
			if result.Winner == match.Side(i) {
				pose = 180
			}
		}
		if err := programs[i].selfState(rt, pose); err != nil {
			return err
		}
		state.Fighters[i] = rt.Context.FighterState
	}
	return nil
}

// Settled reports a fighter at rest for RoundState 4 (system.go:3446-3460):
// knocked out, or in control, idle and standing.
func Settled(rt *FighterRuntime) bool {
	c := &rt.Context
	if c.Health <= 0 {
		return true
	}
	return c.Ctrl && c.MoveType == 'I' && c.StateType == 'S'
}

// IntroAsserted reports a fighter asserting intro (GSF_intro), which holds
// the round clock in RoundState 1.
func IntroAsserted(rt *FighterRuntime) bool {
	return rt.Context.Asserted.Has("intro")
}

// selfState enters state number as a ChangeState with no anim or ctrl does,
// the character's own state; a state it lacks leaves it where it is.
func (p FighterProgram) selfState(rt *FighterRuntime, number int) error {
	def, ok := p.stateHeader(number)
	if !ok {
		return nil
	}
	c := &rt.Context
	c.PrevStateNo, c.StateNo = c.StateNo, number
	_, err := enterState(c, def)
	return err
}
