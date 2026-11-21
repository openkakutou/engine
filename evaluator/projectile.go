package evaluator

// ProjectileSpawn is a Projectile controller that ran: what the engine needs
// to create the projectile, its numbers evaluated when the controller ran
// (Ikemen GO projectileSub, compiler_functions.go:2404-2585). Velocities and
// offsets are in the content's units: x forward, y down.
type ProjectileSpawn struct {
	ID, Anim, HitAnim, RemAnim, CancelAnim int
	// Hits and Priority are projhits and projpriority (1 each): how many
	// trades with other projectiles it takes, and how many it wins first.
	Hits, Priority int
	// RemoveTime is projremovetime: -1 never, -2 when the animation ends.
	RemoveTime                      int
	Velocity, Accel, VelMul, RemVel [2]float64
	Offset                          [2]float64
	// HitDef is the projectile's HitDef parameters, as written.
	HitDef map[string]string
}

// ProjContact is when a player's projectiles with ID last hit and were last
// guarded, in GameTime ticks; 0 is never.
type ProjContact struct {
	ID                     int
	HitTick, Guard, Cancel int
}

// projTime is the ticks since the player's projectile id hit (guarded for
// "guarded", either for "contact", cancelled by another projectile for
// "cancel"), -1 when it never did (Ikemen GO projHitTime and siblings,
// char.go:6306-6330); the tick after the contact reads 0.
func projTime(ctx *Context, kind string, id int) int {
	last := 0
	for _, c := range ctx.ProjContacts {
		if c.ID != id {
			continue
		}
		switch kind {
		case "hit":
			last = c.HitTick
		case "guarded":
			last = c.Guard
		case "cancel":
			last = c.Cancel
		default:
			last = max(c.HitTick, c.Guard)
		}
	}
	if last == 0 {
		return -1
	}
	return ctx.GameTime - last - 1
}
