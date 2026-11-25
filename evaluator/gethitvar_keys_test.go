package evaluator

import "testing"

// Every numeric GetHitVar key reads its own field: each field holds a value
// no other field holds, so a key wired to the wrong field reads a wrong one.
func TestEval_GetHitVar_EachNumericKeyReadsItsOwnField(t *testing.T) {
	ctx := Context{GetHit: GetHit{
		Active: true, HitShakeTime: 1, HitTime: 2, SlideTime: 3, AnimType: 4,
		GroundType: 5, AirType: 6, CtrlTime: 7, Yvel: 8.5, Yaccel: 9.25,
		FallYvel: 10.5, FallRecoverTime: 11, FallTime: 12, FallCount: 13,
		HitCount: 14, Damage: 15, Xvel: 16.5, FallXvel: 17.5, HasFallXvel: true,
		FallDamage: 18, FallEnvShake: EnvShake{Time: 19, Freq: 20.5, Ampl: 21, Phase: 22.5},
		DownRecoverTime: 23, HitID: 24,
	}}
	for _, c := range []struct {
		key  string
		want float64
	}{
		{"hitshaketime", 1}, {"hittime", 2}, {"slidetime", 3}, {"animtype", 4},
		{"groundtype", 5}, {"airtype", 6}, {"ctrltime", 7}, {"yvel", 8.5},
		{"yaccel", 9.25}, {"fall.yvel", 10.5}, {"fall.recovertime", 11},
		{"fall.time", 12}, {"fallcount", 13}, {"hitcount", 14}, {"damage", 15},
		{"xvel", 16.5}, {"fall.xvel", 17.5}, {"fall.damage", 18},
		{"fall.envshake.time", 19}, {"fall.envshake.freq", 20.5},
		{"fall.envshake.ampl", 21}, {"fall.envshake.phase", 22.5},
		{"down.recovertime", 23}, {"hitid", 24}, {"chainid", 24},
	} {
		if got := mustEval(t, "GetHitVar("+c.key+")", ctx).Float(); got != c.want {
			t.Errorf("GetHitVar(%s) = %v, want %v", c.key, got, c.want)
		}
	}
}

// Every boolean GetHitVar key reads its own flag: set alone, it reads 1 and
// the other flags read 0.
func TestEval_GetHitVar_EachBooleanKeyReadsItsOwnFlag(t *testing.T) {
	keys := map[string]GetHit{
		"fall":         {Active: true, Fall: true},
		"fall.recover": {Active: true, FallRecover: true},
		"fall.kill":    {Active: true, FallKill: true},
		"guarded":      {Active: true, Guarded: true},
		"isbound":      {Active: true, BindTime: 3},
	}
	for set, g := range keys {
		ctx := Context{GetHit: g}
		for key := range keys {
			want := key == set
			if got := mustEval(t, "GetHitVar("+key+")", ctx).Bool(); got != want {
				t.Errorf("with only %s set: GetHitVar(%s) = %v, want %v", set, key, got, want)
			}
		}
	}
}
