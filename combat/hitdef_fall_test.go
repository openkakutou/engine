package combat

import "testing"

// char.go:11680-11838 copies these HitDef parameters into the get-hit
// variables; Ikemen GO's defaults: fall.xvelocity unset (NaN), fall.damage 0,
// fall.kill 1 (char.go:950-1000).
func TestParseHitDef_VelocityXAndFallParameters(t *testing.T) {
	p := ParseHitDef(map[string]string{
		"ground.velocity": "-4, -3", "air.velocity": "-2.5, -6",
		"fall.xvelocity": "-1.5", "fall.damage": "30", "fall.kill": "0",
	})
	if p.GroundVelocityX != -4 || p.AirVelocityX != -2.5 {
		t.Errorf("x velocities = %v, %v; want -4, -2.5", p.GroundVelocityX, p.AirVelocityX)
	}
	if !p.HasFallXVelocity || p.FallXVelocity != -1.5 || p.FallDamage != 30 || p.FallKill {
		t.Errorf("fall = %v %d %v, want -1.5 30 false", p.FallXVelocity, p.FallDamage, p.FallKill)
	}
	d := ParseHitDef(map[string]string{})
	if d.HasFallXVelocity || d.FallDamage != 0 || !d.FallKill || d.GroundVelocityX != 0 {
		t.Errorf("defaults = %v %d %v %v, want unset 0 true 0", d.HasFallXVelocity, d.FallDamage, d.FallKill, d.GroundVelocityX)
	}
}

// char.go:8889-8917 computeDamage: damage times the attack multiplier over
// the final defence, at least 1 when positive, rounded.
func TestScaledDamage(t *testing.T) {
	for _, c := range []struct {
		damage int
		mul    float64
		want   int
	}{
		{26, 0, 26}, {26, 1, 26}, {26, 0.5, 13}, {25, 0.5, 13}, {1, 0.1, 1}, {0, 2, 0}, {30, 1.0 / 1.5, 20},
	} {
		if got := scaledDamage(c.damage, c.mul); got != c.want {
			t.Errorf("scaledDamage(%d, %v) = %d, want %d", c.damage, c.mul, got, c.want)
		}
	}
}
