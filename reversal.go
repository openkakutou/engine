package engine

import (
	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/combat"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/hitdetect"
	"github.com/openkakutou/engine/v2/match"
)

// reverse lands the reverser's ReversalDef on the attacker's HitDef when it
// reverses it and their attack boxes touch (char.go:11069-11082, 11200-
// 11240, 11500-11510): the attacker's HitDef is spent and its move counts
// as reversed (MoveReversed); the reverser gets its pausetime, spark and
// hitsound, enters p1stateno, and sends the attacker to p2stateno in its own
// states; the ReversalDef is spent.
// shortcut: reversal against helpers and projectiles, and the HitDef
// priority rules, are not modelled.
func reverse(state *match.MatchState, frames [2]air.Frame, programs *[2]FighterProgram, runtimes *[2]FighterRuntime, reverser match.Side, cfg TickConfig, hitSounds *[2][]evaluator.SoundEvent) error {
	rt := &runtimes[reverser]
	att := &runtimes[1-reverser]
	rev := &rt.Context
	if rev.ReversalAttr == 0 || !hitDefReady(att) || !evaluator.ReversalMatches(rev.ReversalAttr, att.Context.HitDefAttr) {
		return nil
	}
	if !hitdetect.BoxesOverlap(rev.Position, rev.Facing, frames[reverser].Clsn1, att.Context.Position, att.Context.Facing, frames[1-reverser].Clsn1) {
		return nil
	}
	hitDef := resolveHitDef(cns.Controller{Type: "ReversalDef", Parameters: rev.Reversal}, rev)
	params := combat.ParseHitDef(hitDef.Parameters)
	rev.ReversalAttr, rev.Reversal = 0, nil
	att.HitDefHit = true
	att.Context.MoveReversedTime = 1
	rev.HasTarget = true
	rt.HitPause = params.PauseTime[0]
	att.HitPause = params.PauseTime[1]
	rev.Events = append(rev.Events, hitSpark(rev, &att.Context, hitDef.Parameters, false))
	if snd, ok := hitSound(hitDef.Parameters["hitsound"], rev); ok {
		hitSounds[reverser] = append(hitSounds[reverser], snd)
	}
	switch params.P2Facing {
	case 1:
		att.Context.Facing = 1 - rev.Facing
	case -1:
		att.Context.Facing = rev.Facing
	}
	if params.P1Facing < 0 {
		rev.Facing = 1 - rev.Facing
	}
	if params.P1StateNo >= 0 {
		if err := programs[reverser].selfState(rt, params.P1StateNo); err != nil {
			return err
		}
	}
	if params.P2StateNo >= 0 {
		if def, ok := programs[reverser].stateHeader(params.P2StateNo); ok {
			c := &att.Context
			c.PrevStateNo, c.StateNo = c.StateNo, params.P2StateNo
			c.InOpponentStates = true
			c.Ctrl = false
			if _, err := enterState(c, def); err != nil {
				return err
			}
		}
	}
	state.Fighters[reverser] = rev.FighterState
	state.Fighters[1-reverser] = att.Context.FighterState
	return nil
}
