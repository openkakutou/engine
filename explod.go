package engine

import (
	"github.com/openkakutou/character/air"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/match"
)

// explodMax is Ikemen GO's default ExplodMax (defaultConfig.ini): past it an
// Explod controller makes nothing (char.go:7309-7315).
const explodMax = 512

// Explod is a live explod (Ikemen GO Explod, char.go:1876-2330): an
// animation of its owner's (or, Common, of the common fightfx) at a world
// position, Time ticks old. Owner is the serial of the helper that made it,
// 0 for the player.
type Explod struct {
	ID, Anim, AnimTime, Time int
	Common                   bool
	Position                 match.Position
	Facing                   match.Facing
	// VFlip is a negative vfacing: drawn upside down.
	VFlip       bool
	SprPriority int
	OnTop       bool
	Owner       int
	// Bound follows Bind (a FighterState position) while BindTime is not 0.
	BindTime, RemoveTime int
	Rel, Vel, Accel      [2]float64
	PosType              string
	RemoveOnGetHit       bool
	// MovesInPause keeps it going during a pause (a SuperPause's effect, an
	// explod with supermovetime or pausemovetime).
	MovesInPause bool
	// Scale is its drawing scale.
	Scale [2]float64
	// Trans and Alpha are its trans type ("" for its animation's blend) and
	// alpha.
	Trans string
	Alpha [2]int
}

// updateExplods is Ikemen GO's Explod.update (char.go:2135-2310) for every
// live explod: removed past its remove time or, at -2, its animation's end,
// or when its owner is hit with removeongethit; while bound it follows its
// owner, then it moves by its velocity, which grows by its acceleration.
// shortcut: friction and binding to another fighter than the owner (a
// postype other than p1) are not modelled.
func updateExplods(rt *FighterRuntime, prog FighterProgram, paused bool) {
	anims := prog.animationTable()
	kept := rt.Explods[:0]
	for _, e := range rt.Explods {
		// A pause holds every explod but those that move in it.
		if paused && !e.MovesInPause {
			kept = append(kept, e)
			continue
		}
		owner := explodOwner(rt, e.Owner)
		if e.RemoveTime >= 0 && e.Time >= e.RemoveTime {
			continue
		}
		if e.RemoveTime <= -2 && !e.Common && animationEnded(anims[e.Anim], e.AnimTime) {
			continue
		}
		if e.RemoveOnGetHit && owner != nil && owner.MoveType == 'H' && !owner.GetHit.Guarded {
			continue
		}
		if e.BindTime != 0 && owner != nil && e.PosType == "p1" {
			e.Position = explodAt(owner.Position, owner.Facing, e.Rel)
		} else {
			e.Position.X += e.Vel[0] * facingSign(e.Facing)
			e.Position.Y -= e.Vel[1]
			e.Vel[0] += e.Accel[0]
			e.Vel[1] += e.Accel[1]
		}
		if e.BindTime > 0 {
			e.BindTime--
		}
		e.Time++
		e.AnimTime++
		kept = append(kept, e)
	}
	rt.Explods = kept
}

// animationEnded reports an animation played through once (Ikemen GO's
// loopend): never for one holding a frame forever or a missing one, which
// the engine cannot tell from a common one.
func animationEnded(anim air.Animation, animTime int) bool {
	length := animationLength(anim)
	return length > 0 && animTime >= length
}

// explodOwner is the context of the fighter with that serial: the player
// for 0, else one of its helpers; nil once that helper is gone.
func explodOwner(rt *FighterRuntime, serial int) *evaluator.Context {
	if serial == 0 {
		return &rt.Context
	}
	if h := findHelper(rt.Helpers, serial); h != nil {
		return &h.Context
	}
	return nil
}

// explodAt is a p1 explod's place: the fighter plus rel, x toward its
// facing, y up for a negative rel y (char.go:1961-1980).
func explodAt(pos match.Position, facing match.Facing, rel [2]float64) match.Position {
	return match.Position{X: pos.X + rel[0]*facingSign(facing), Y: pos.Y - rel[1]}
}

// spawnExplods turns the Explod and RemoveExplod controllers the owner ran
// this tick into explods (setPos, char.go:1961-2036): p1 at the owner, p2 at
// the opponent, left and right at the screen edges, front and back at the
// edge the owner faces or not, none at the stage position itself; facing
// the owner's way (or the opponent's for p2), turned by facing -1.
// shortcut: screen-space explods do not follow the camera.
func spawnExplods(rt *FighterRuntime, serial int, opponent evaluator.Opponent, screen [2]float64) {
	ctx := explodOwner(rt, serial)
	for _, r := range ctx.RemovedExplods {
		kept := rt.Explods[:0]
		for _, e := range rt.Explods {
			if e.Owner != serial || (r.ID >= 0 && e.ID != r.ID) {
				kept = append(kept, e)
			}
		}
		rt.Explods = kept
	}
	ctx.RemovedExplods = nil
	for _, m := range ctx.ModifiedExplods {
		for i := range rt.Explods {
			if e := &rt.Explods[i]; e.Owner == serial && (m.ID < 0 || e.ID == m.ID) {
				modifyExplod(e, m)
			}
		}
	}
	ctx.ModifiedExplods = nil
	for _, s := range ctx.NewExplods {
		if len(rt.Explods) >= explodMax {
			break
		}
		// Time 1: Ikemen GO updates an explod in the tick it is made
		// (char.go:2147-2154), so it lives removetime ticks.
		e := Explod{Time: 1, ID: s.ID, Anim: s.Anim, Common: s.Common, Owner: serial, BindTime: s.BindTime, RemoveTime: s.RemoveTime,
			Rel: s.Pos, Vel: s.Vel, Accel: s.Accel, PosType: s.PosType, RemoveOnGetHit: s.RemoveOnGetHit,
			SprPriority: s.SprPriority, OnTop: s.OnTop, VFlip: s.VFacing < 0, Facing: ctx.Facing, MovesInPause: s.MovesInPause, Scale: s.Scale, Trans: s.Trans, Alpha: s.Alpha}
		switch s.PosType {
		case "p1":
			e.Position = explodAt(ctx.Position, ctx.Facing, s.Pos)
		case "p2":
			e.Facing = opponent.Facing
			e.Position = explodAt(opponent.Position, opponent.Facing, s.Pos)
		case "left", "right", "front", "back":
			right := s.PosType == "right" || (s.PosType == "front") == (ctx.Facing == match.FacingRight)
			if s.PosType == "left" {
				right = false
			}
			x := screen[0] + s.Pos[0]
			if right {
				x = screen[1] + s.Pos[0]
			}
			e.Position = match.Position{X: x, Y: -s.Pos[1]}
		default:
			e.Position = match.Position{X: s.Pos[0], Y: -s.Pos[1]}
		}
		if s.Facing < 0 {
			e.Facing = 1 - e.Facing
		}
		rt.Explods = append(rt.Explods, e)
	}
	ctx.NewExplods = nil
}

// modifyExplod gives e the parameters a ModifyExplod wrote.
func modifyExplod(e *Explod, m evaluator.ExplodModification) {
	if m.SetPosType {
		e.PosType = m.PosType
	}
	if m.SetPos {
		e.Rel = m.Pos
	}
	if m.SetVel {
		e.Vel = m.Vel
	}
	if m.SetAccel {
		e.Accel = m.Accel
	}
	if m.SetFacing && m.Facing < 0 {
		e.Facing = 1 - e.Facing
	}
	if m.SetVFacing {
		e.VFlip = m.VFacing < 0
	}
	if m.SetBindTime {
		e.BindTime = relativeTime(e, m.BindTime, m.Relative)
	}
	if m.SetRemoveTime {
		e.RemoveTime = relativeTime(e, m.RemoveTime, m.Relative)
	}
	if m.SetSprPriority {
		e.SprPriority = m.SprPriority
	}
	if m.SetOnTop {
		e.OnTop = m.OnTop
	}
	if m.SetRemoveOnGetHit {
		e.RemoveOnGetHit = m.RemoveOnGetHit
	}
	if m.SetScale {
		e.Scale = m.Scale
	}
	if m.SetTrans {
		e.Trans, e.Alpha = m.Trans, m.Alpha
	}
	if m.SetMovesInPause {
		e.MovesInPause = m.MovesInPause
	}
	if m.SetAnim {
		e.Anim, e.Common, e.AnimTime = m.Anim, m.Common, 0
	}
}

// relativeTime is a modified bindtime or removetime: from the explod's age
// for an Ikemen GO character's positive one.
func relativeTime(e *Explod, t int, relative bool) int {
	if relative && t > 0 {
		return e.Time + t
	}
	return t
}

// explodIDs are owner's explods' ids, for NumExplod.
func explodIDs(explods []Explod, owner int) []int {
	var ids []int
	for _, e := range explods {
		if e.Owner == owner {
			ids = append(ids, e.ID)
		}
	}
	return ids
}

// tickExplods updates the player's live explods, then makes those the player
// and its helpers asked for this tick, and leaves each its explod ids.
func tickExplods(rt *FighterRuntime, prog FighterProgram, opponent evaluator.Opponent, screen [2]float64, paused bool) {
	// Nothing to do allocates nothing: most ticks of most characters.
	if len(rt.Explods) == 0 && len(rt.Context.NewExplods) == 0 && len(rt.Context.RemovedExplods) == 0 && len(rt.Context.ModifiedExplods) == 0 && len(rt.Helpers) == 0 {
		rt.Context.ExplodIDs = nil
		return
	}
	updateExplods(rt, prog, paused)
	spawnExplods(rt, 0, opponent, screen)
	for i := range rt.Helpers {
		spawnExplods(rt, rt.Helpers[i].Serial, opponent, screen)
	}
	rt.Context.ExplodIDs = explodIDs(rt.Explods, 0)
	for i := range rt.Helpers {
		rt.Helpers[i].Context.ExplodIDs = explodIDs(rt.Explods, rt.Helpers[i].Serial)
	}
}
