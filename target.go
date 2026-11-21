package engine

import (
	"github.com/openkakutou/engine/v2/evaluator"
)

// applyTargetOps applies the Target* controllers the attacker ran this tick
// to its target, the defender while the attacker has hit it and it is still
// being hit (Ikemen GO's getTarget, char.go:10387-10404; the controllers in
// bytecode.go): TargetBind places it at the attacker plus pos, x toward the
// attacker's facing, and binds it for time ticks; TargetFacing turns it the
// attacker's way (value > 0) or away; TargetState sends it into the
// attacker's state value (a custom state); TargetLifeAdd adds value to its
// life, scaled by its defence unless absolute, never killing it without
// kill; TargetPowerAdd adds power; TargetVelSet / TargetVelAdd set or add to
// its velocity, x toward its facing; TargetDrop lets it go.
func applyTargetOps(attacker, defender *evaluator.Context, attackerProg FighterProgram) error {
	ops := attacker.TargetOps
	attacker.TargetOps = nil
	for _, op := range ops {
		if !attacker.HasTarget || defender.MoveType != 'H' {
			return nil
		}
		switch op.Kind {
		case "targetbind":
			defender.Position.X = attacker.Position.X + op.X*facingSign(attacker.Facing)
			defender.Position.Y = attacker.Position.Y - op.Y
			defender.GetHit.BindTime = max(op.Time, 1)
		case "targetfacing":
			defender.Facing = attacker.Facing
			if op.Value < 0 {
				defender.Facing = 1 - attacker.Facing
			}
		case "targetstate":
			def, ok := attackerProg.stateHeader(op.Value)
			if !ok {
				continue
			}
			defender.PrevStateNo, defender.StateNo = defender.StateNo, op.Value
			defender.InOpponentStates = true
			if _, err := enterState(defender, def); err != nil {
				return err
			}
		case "targetlifeadd":
			add := float64(op.Value)
			if !op.Absolute {
				if def := defender.Consts.Lookup("data.defence").Number; def > 0 {
					add /= def / 100
				}
			}
			life := defender.Health + int(add)
			if !op.Kill && life <= 0 {
				life = 1
			}
			defender.Health = max(life, 0)
		case "targetpoweradd":
			defender.Power = max(defender.Power+op.Value, 0)
		case "targetvelset", "targetveladd":
			x := op.X * facingSign(defender.Facing)
			if op.Kind == "targetveladd" {
				x += defender.Velocity.X
			}
			if op.HasX {
				defender.Velocity.X = x
			}
			if op.HasY {
				y := -op.Y
				if op.Kind == "targetveladd" {
					y += defender.Velocity.Y
				}
				defender.Velocity.Y = y
			}
		case "targetdrop":
			attacker.HasTarget = false
			defender.GetHit.BindTime = 0
		}
	}
	return nil
}
