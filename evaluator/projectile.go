package evaluator

// ProjectileSpawn is a Projectile controller that ran: what the engine needs
// to create the projectile, its numbers evaluated when the controller ran
// (Ikemen GO projectileSub, compiler_functions.go:2404-2585). Velocities and
// offsets are in the content's units: x forward, y down.
type ProjectileSpawn struct {
	ID, Anim, HitAnim, RemAnim int
	// RemoveTime is projremovetime: -1 never, -2 when the animation ends.
	RemoveTime                      int
	Velocity, Accel, VelMul, RemVel [2]float64
	Offset                          [2]float64
	// HitDef is the projectile's HitDef parameters, as written.
	HitDef map[string]string
}
