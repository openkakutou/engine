package engine

import (
	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/combat"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/hitdetect"
	"github.com/openkakutou/engine/v2/match"
	"slices"
)

// ProjectileStatus is a projectile's life stage, Ikemen GO's ProjActive,
// ProjHit and ProjRem.
type ProjectileStatus int

const (
	ProjectileActive ProjectileStatus = iota
	ProjectileHit
	ProjectileRemoved
)

// projectileBound is how far past the screen or the stage a projectile flies
// before it is removed, Ikemen GO's default edgebound and stagebound
// (char.go:2749-2750).
const projectileBound = 40

// Projectile is a fighter's live projectile (Ikemen GO Projectile,
// char.go:2719-2937): where it is, which animation it plays, and the HitDef
// it lands. Velocities are in the content's units (x forward, y down).
type Projectile struct {
	ID       int
	Anim     int
	AnimTime int
	Position match.Position
	Facing   match.Facing
	Status   ProjectileStatus
	// RemoveDone is set once a hit or removed projectile has switched to its
	// hit or remove animation and velocity.
	RemoveDone                      bool
	RemoveTime                      int
	HitAnim, RemAnim                int
	Velocity, Accel, VelMul, RemVel [2]float64
	HitDef                          cns.Controller
	contact                         bool
}

// spawnProjectiles turns the Projectile controllers the thrower (the player
// or one of its helpers) ran this tick into the player's projectiles, at the
// thrower's position plus offset, facing its way (postype p1,
// commitProjectile).
func spawnProjectiles(thrower *evaluator.Context, rt *FighterRuntime) {
	for _, s := range thrower.NewProjectiles {
		pos := thrower.Position
		pos.X += s.Offset[0] * facingSign(thrower.Facing)
		pos.Y -= s.Offset[1]
		rt.Projectiles = append(rt.Projectiles, Projectile{
			ID: s.ID, Anim: s.Anim, Position: pos, Facing: thrower.Facing,
			RemoveTime: s.RemoveTime, HitAnim: s.HitAnim, RemAnim: s.RemAnim,
			Velocity: s.Velocity, Accel: s.Accel, VelMul: s.VelMul, RemVel: s.RemVel,
			HitDef: cns.Controller{Type: "HitDef", Parameters: s.HitDef},
		})
	}
	thrower.NewProjectiles = nil
}

// updateProjectiles is Ikemen GO's Projectile.update (char.go:2804-2930):
// an active projectile past its remove time or its bounds is removed; a hit
// or removed one switches to its hit or remove animation (none: it goes at
// once) and its remove velocity, and goes when that animation ends; then
// every projectile moves and its velocity takes its acceleration.
// shortcut: heightbound, depth and hit pause are not read.
func updateProjectiles(rt *FighterRuntime, prog FighterProgram, screen [2]float64, cfg TickConfig) {
	anims := prog.animationTable()
	kept := rt.Projectiles[:0]
	for _, p := range rt.Projectiles {
		if p.Status == ProjectileActive && projectileOut(p, screen, cfg) {
			p.Status = ProjectileRemoved
		}
		if p.Status != ProjectileActive && !p.RemoveDone {
			next := p.RemAnim
			if p.Status == ProjectileHit {
				next = p.HitAnim
			}
			if next == -1 {
				continue
			}
			if _, ok := anims[next]; ok && next != p.Anim {
				p.Anim, p.AnimTime = next, 0
			}
			p.RemoveDone = true
			p.Velocity, p.Accel, p.VelMul = p.RemVel, [2]float64{}, [2]float64{1, 1}
		}
		if p.Status != ProjectileActive {
			length := animationLength(anims[p.Anim])
			if length <= 0 || p.AnimTime >= length {
				continue
			}
		}
		p.Position.X += p.Velocity[0] * facingSign(p.Facing)
		p.Position.Y -= p.Velocity[1]
		for i := range p.Velocity {
			p.Velocity[i] = (p.Velocity[i] + p.Accel[i]) * p.VelMul[i]
		}
		if p.Velocity[0] < 0 {
			p.Facing = 1 - p.Facing
			p.Velocity[0], p.Accel[0] = -p.Velocity[0], -p.Accel[0]
		}
		kept = append(kept, p)
	}
	rt.Projectiles = kept
}

// projectileOut reports a projectile past its remove time, the screen edges
// or, moving outward, the stage edges.
func projectileOut(p Projectile, screen [2]float64, cfg TickConfig) bool {
	if p.RemoveTime == 0 {
		return true
	}
	x := p.Position.X
	if x < screen[0]-projectileBound || x > screen[1]+projectileBound {
		return true
	}
	if cfg.Bounds == nil {
		return false
	}
	dir := p.Velocity[0] * facingSign(p.Facing)
	if dir < 0 && x < float64(cfg.Bounds.Left)-projectileBound {
		return true
	}
	return dir > 0 && x > float64(cfg.Bounds.Right)+projectileBound
}

// tickProjectiles is Ikemen GO's Projectile.tick (char.go:3182-3215): a
// projectile that hit this tick becomes a hit projectile, the remove time
// counts down and the animation advances.
// shortcut: projhits above 1 and projremove 0 are not read: a hit ends it.
func tickProjectiles(rt *FighterRuntime) {
	for i := range rt.Projectiles {
		p := &rt.Projectiles[i]
		if p.contact {
			p.contact = false
			p.Status = ProjectileHit
		}
		if p.RemoveTime > 0 {
			p.RemoveTime--
		}
		p.AnimTime++
	}
}

// hitWithProjectiles lands attacker's active projectiles whose hit boxes
// touch the defender's hurt boxes, each like a HitDef, the first one
// touching only: a defender is hit once a tick.
// shortcut: the owner's contact counters, the projectile's own hit pause
// and projectile-against-projectile trades are not modelled; the defender
// is knocked away along the owner's facing.
func hitWithProjectiles(state *match.MatchState, programs *[2]FighterProgram, runtimes *[2]FighterRuntime, attacker match.Side, defFrameClsn2 []air.ClsnBox, cfg TickConfig, hitSounds *[2][]evaluator.SoundEvent) error {
	rt := &runtimes[attacker]
	defender := &runtimes[1-attacker].Context
	anims := programs[attacker].animationTable()
	for i := range rt.Projectiles {
		p := &rt.Projectiles[i]
		if p.Status != ProjectileActive {
			continue
		}
		frame := currentFrame(anims[p.Anim], p.AnimTime)
		if !hitBoxesTouch(p.Position, p.Facing, frame.Clsn1, defender, defFrameClsn2) {
			continue
		}
		attr, err := evaluator.ParseAttr(p.HitDef.Parameters["attr"], true)
		if err != nil || !evaluator.HittableBy(defender.HitBy, attr) {
			continue
		}
		from := rt.Context
		from.Position, from.Facing = p.Position, p.Facing
		owner := ownerContact{rt.HitPause, rt.Context.MoveContactTime, rt.Context.ContactGuarded, rt.Context.HasTarget}
		hit, guarded, params := strike(state, runtimes, attacker, p.HitDef, &from, cfg, hitSounds)
		if !hit.Hit {
			continue
		}
		p.contact = true
		rt.Context.ProjContacts = recordProjContact(rt.Context.ProjContacts, p.ID, cfg.Tick, guarded)
		err = applyHitReaction(programs, runtimes, attacker, params, hit, guarded)
		rt.HitPause, rt.Context.MoveContactTime, rt.Context.ContactGuarded, rt.Context.HasTarget = owner.pause, owner.moveContact, owner.guarded, owner.target
		return err
	}
	return nil
}

// ownerContact is the owner's contact state a projectile hit leaves alone.
type ownerContact struct {
	pause, moveContact int
	guarded, target    bool
}

// strike lands hitDef from the attacker (from is how it is seen: the
// fighter or its projectile) on the defender: guard or hit, damage, combo,
// spark and sound. It reports the hit, whether it was guarded and the parsed
// HitDef.
func strike(state *match.MatchState, runtimes *[2]FighterRuntime, attacker match.Side, hitDef cns.Controller, from *evaluator.Context, cfg TickConfig, hitSounds *[2][]evaluator.SoundEvent) (combat.HitResult, bool, combat.HitDefParams) {
	rt := &runtimes[attacker]
	defender := &runtimes[1-attacker].Context
	params := combat.ParseHitDef(hitDef.Parameters)
	damage := combat.DamageParams{Damage: hitDef.Parameters["damage"], Multiplier: damageMultiplier(from, defender)}
	_, guardDamage := combat.Damages(damage)
	damage.Guarded = guards(from, defender, params, guardDamage)
	lifeBefore := state.Fighters[1-attacker].Health
	contact := []hitdetect.HitEvent{{Attacker: attacker, Defender: 1 - attacker}}
	newState, newCombo, hit := combat.ApplyHits(*state, contact, attacker, damage, rt.Combo, cfg.Tick, cfg.ComboWindow)
	if damage.Guarded && hit.Hit {
		newState.Fighters[1-attacker].Health = lifeBefore - guardChip(guardDamage, lifeBefore, params)
	}
	*state = newState
	if !damage.Guarded {
		rt.Combo = newCombo
	}
	if !hit.Hit {
		return hit, damage.Guarded, params
	}
	rt.Context.Events = append(rt.Context.Events, hitSpark(from, defender, hitDef.Parameters, damage.Guarded))
	key := "hitsound"
	if damage.Guarded {
		key = "guardsound"
	}
	if snd, ok := hitSound(hitDef.Parameters[key], from); ok {
		hitSounds[attacker] = append(hitSounds[attacker], snd)
	}
	return hit, damage.Guarded, params
}

// animationLength is the ticks anim plays before it ends, 0 or less when it
// holds a frame forever (a frame time of -1).
func animationLength(anim air.Animation) int {
	total := 0
	for _, f := range anim.Frames {
		if f.Time < 0 {
			return -1
		}
		total += f.Time
	}
	return total
}

// hitBoxesTouch reports whether hit boxes at pos, facing touch the
// defender's hurt boxes.
func hitBoxesTouch(pos match.Position, facing match.Facing, clsn1 []air.ClsnBox, defender *evaluator.Context, clsn2 []air.ClsnBox) bool {
	return hitdetect.BoxesOverlap(pos, facing, clsn1, defender.Position, defender.Facing, clsn2)
}

// projectileIDs are the ids of projectiles, nil for none.
func projectileIDs(projectiles []Projectile) []int {
	var ids []int
	for _, p := range projectiles {
		ids = append(ids, p.ID)
	}
	return ids
}

// recordProjContact is contacts with projectile id's hit (or guard) at tick,
// in a fresh slice: the one handed in belongs to the tick before.
func recordProjContact(contacts []evaluator.ProjContact, id, tick int, guarded bool) []evaluator.ProjContact {
	out := slices.Clone(contacts)
	i := slices.IndexFunc(out, func(c evaluator.ProjContact) bool { return c.ID == id })
	if i < 0 {
		out = append(out, evaluator.ProjContact{ID: id})
		i = len(out) - 1
	}
	if guarded {
		out[i].Guard = tick
	} else {
		out[i].HitTick = tick
	}
	return out
}
