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

// char.go:703-710, 852-855: fall.envshake.time (0), freq (60), ampl and
// phase.
func TestParseHitDef_FallEnvShake(t *testing.T) {
	p := ParseHitDef(map[string]string{"fall.envshake.time": "15", "fall.envshake.freq": "100", "fall.envshake.ampl": "-6", "fall.envshake.phase": "90"})
	if p.FallEnvShakeTime != 15 || p.FallEnvShakeFreq != 100 || p.FallEnvShakeAmpl != -6 || p.FallEnvShakePhase != 90 {
		t.Errorf("fall.envshake = %d %v %v %v", p.FallEnvShakeTime, p.FallEnvShakeFreq, p.FallEnvShakeAmpl, p.FallEnvShakePhase)
	}
	d := ParseHitDef(nil)
	if d.FallEnvShakeTime != 0 || d.FallEnvShakeFreq != 60 {
		t.Errorf("defaults = %d %v, want 0 60", d.FallEnvShakeTime, d.FallEnvShakeFreq)
	}
}

// bytecode.go:142-167 hitFlagMask and char.go:779-789, 933-952: guardflag
// letters H, L, M (H and L), A; guard.pausetime follows pausetime;
// guard.velocity follows ground.velocity x; airguard.velocity is
// air.velocity x 1.5 and y 0.5; guard.hittime, slidetime and ctrltime are
// left unset (-1) for the engine to chain by character version.
func TestParseHitDef_GuardParameters(t *testing.T) {
	p := ParseHitDef(map[string]string{"guardflag": "MA", "pausetime": "12, 11", "ground.velocity": "-4, 0", "air.velocity": "-2, -4"})
	if p.GuardFlag != GuardHigh|GuardLow|GuardAir {
		t.Errorf("guardflag MA = %b", p.GuardFlag)
	}
	if p.GuardPauseTime != [2]int{12, 11} || p.GuardVelocityX != -4 || p.AirGuardVelocityX != -3 || p.AirGuardVelocityY != -2 {
		t.Errorf("guard = %v %v %v %v", p.GuardPauseTime, p.GuardVelocityX, p.AirGuardVelocityX, p.AirGuardVelocityY)
	}
	if p.GuardHitTime != -1 || p.GuardSlideTime != -1 || p.GuardCtrlTime != -1 || !p.GuardKill {
		t.Errorf("unset guard times = %d %d %d, kill %v", p.GuardHitTime, p.GuardSlideTime, p.GuardCtrlTime, p.GuardKill)
	}
	q := ParseHitDef(map[string]string{"guardflag": "l", "guard.pausetime": "3, 4", "guard.hittime": "9", "guard.velocity": "-7", "guard.kill": "0"})
	if q.GuardFlag != GuardLow || q.GuardPauseTime != [2]int{3, 4} || q.GuardHitTime != 9 || q.GuardVelocityX != -7 || q.GuardKill {
		t.Errorf("explicit guard = %+v", q)
	}
}
