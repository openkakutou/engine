package combat

import (
	"testing"

	"github.com/openkakutou/engine/v2/hitdetect"
	"github.com/openkakutou/engine/v2/match"
)

func newTestState(t *testing.T, p1Health, p2Health int) match.MatchState {
	t.Helper()
	ms, err := match.NewMatchState(
		1, 0,
		match.FighterState{Side: match.SideP1, Health: p1Health},
		match.FighterState{Side: match.SideP2, Health: p2Health},
	)
	if err != nil {
		t.Fatalf("failed to build match state: %v", err)
	}
	return *ms
}

func hitDefWithDamage(damage string) DamageParams {
	return DamageParams{Damage: damage}
}

func TestApplyHits_ReducesDefenderHealthByDeclaredDamage(t *testing.T) {
	state := newTestState(t, 1000, 1000)
	events := []hitdetect.HitEvent{
		{Attacker: match.SideP1, Defender: match.SideP2},
	}

	newState, _, result := ApplyHits(
		state, events, match.SideP1, hitDefWithDamage("30"), ComboState{}, 1, 60,
	)

	if got := newState.Fighter(match.SideP2).Health; got != 970 {
		t.Errorf("defender health = %d, want 970", got)
	}
	if !result.Hit {
		t.Error("expected result.Hit to be true")
	}
	if result.Damage != 30 {
		t.Errorf("result.Damage = %d, want 30", result.Damage)
	}
	if result.HealthBefore != 1000 || result.HealthAfter != 970 {
		t.Errorf("result health before/after = %d/%d, want 1000/970", result.HealthBefore, result.HealthAfter)
	}
}

func TestApplyHits_LeavesStateUnchanged_WhenNoEventMatchesAttacker(t *testing.T) {
	state := newTestState(t, 1000, 1000)
	events := []hitdetect.HitEvent{
		{Attacker: match.SideP2, Defender: match.SideP1},
	}
	combo := ComboState{Count: 3, LastHitTick: 5, Active: true}

	newState, newCombo, result := ApplyHits(
		state, events, match.SideP1, hitDefWithDamage("30"), combo, 10, 60,
	)

	if newState.Fighter(match.SideP2).Health != 1000 {
		t.Errorf("defender health changed to %d, want unchanged 1000", newState.Fighter(match.SideP2).Health)
	}
	if newCombo != combo {
		t.Errorf("combo changed to %+v, want unchanged %+v", newCombo, combo)
	}
	if result.Hit {
		t.Error("expected result.Hit to be false")
	}
}

func TestApplyHits_NeverReducesHealthBelowZero_OnAnOverkillHit(t *testing.T) {
	state := newTestState(t, 10, 10)
	events := []hitdetect.HitEvent{
		{Attacker: match.SideP1, Defender: match.SideP2},
	}

	newState, _, result := ApplyHits(
		state, events, match.SideP1, hitDefWithDamage("9999"), ComboState{}, 1, 60,
	)

	if got := newState.Fighter(match.SideP2).Health; got != 0 {
		t.Errorf("defender health = %d, want floored at 0", got)
	}
	if result.HealthAfter != 0 {
		t.Errorf("result.HealthAfter = %d, want 0", result.HealthAfter)
	}
}

func TestApplyHits_FallsBackToDefaultDamage_WhenDamageParameterIsMissing(t *testing.T) {
	state := newTestState(t, 1000, 1000)
	events := []hitdetect.HitEvent{
		{Attacker: match.SideP1, Defender: match.SideP2},
	}
	noDamageParam := DamageParams{}

	newState, _, result := ApplyHits(
		state, events, match.SideP1, noDamageParam, ComboState{}, 1, 60,
	)

	if result.Damage != DefaultDamage {
		t.Errorf("result.Damage = %d, want default %d", result.Damage, DefaultDamage)
	}
	if got := newState.Fighter(match.SideP2).Health; got != 1000-DefaultDamage {
		t.Errorf("defender health = %d, want %d", got, 1000-DefaultDamage)
	}
}

func TestApplyHits_FallsBackToDefaultDamage_WhenDamageParameterIsNotNumeric(t *testing.T) {
	state := newTestState(t, 1000, 1000)
	events := []hitdetect.HitEvent{
		{Attacker: match.SideP1, Defender: match.SideP2},
	}

	_, _, result := ApplyHits(
		state, events, match.SideP1, hitDefWithDamage("not-a-number"), ComboState{}, 1, 60,
	)

	if result.Damage != DefaultDamage {
		t.Errorf("result.Damage = %d, want default %d", result.Damage, DefaultDamage)
	}
}

func TestApplyHits_CountsOneHit_ForMultipleOverlappingBoxPairsAgainstTheSameDefender(t *testing.T) {
	state := newTestState(t, 1000, 1000)
	events := []hitdetect.HitEvent{
		{Attacker: match.SideP1, Defender: match.SideP2},
		{Attacker: match.SideP1, Defender: match.SideP2},
		{Attacker: match.SideP1, Defender: match.SideP2},
	}

	newState, newCombo, result := ApplyHits(
		state, events, match.SideP1, hitDefWithDamage("30"), ComboState{}, 1, 60,
	)

	if got := newState.Fighter(match.SideP2).Health; got != 970 {
		t.Errorf("defender health = %d, want 970 (damage applied once, not 3x)", got)
	}
	if newCombo.Count != 1 {
		t.Errorf("combo count = %d, want 1 (one hit, not 3)", newCombo.Count)
	}
	if result.ComboCount != 1 {
		t.Errorf("result.ComboCount = %d, want 1", result.ComboCount)
	}
}

func TestApplyHits_IncrementsComboWithinTheWindow(t *testing.T) {
	state := newTestState(t, 1000, 1000)
	events := []hitdetect.HitEvent{{Attacker: match.SideP1, Defender: match.SideP2}}
	comboWindow := 60

	_, combo1, _ := ApplyHits(state, events, match.SideP1, hitDefWithDamage("10"), ComboState{}, 100, comboWindow)
	if combo1.Count != 1 {
		t.Fatalf("first hit combo count = %d, want 1", combo1.Count)
	}

	_, combo2, result2 := ApplyHits(state, events, match.SideP1, hitDefWithDamage("10"), combo1, 130, comboWindow)
	if combo2.Count != 2 {
		t.Errorf("second hit combo count = %d, want 2 (within window)", combo2.Count)
	}
	if result2.ComboCount != 2 {
		t.Errorf("result2.ComboCount = %d, want 2", result2.ComboCount)
	}
}

func TestApplyHits_ResetsComboToOne_WhenTheWindowLapses(t *testing.T) {
	state := newTestState(t, 1000, 1000)
	events := []hitdetect.HitEvent{{Attacker: match.SideP1, Defender: match.SideP2}}
	comboWindow := 60

	_, combo1, _ := ApplyHits(state, events, match.SideP1, hitDefWithDamage("10"), ComboState{}, 100, comboWindow)

	_, combo2, result2 := ApplyHits(state, events, match.SideP1, hitDefWithDamage("10"), combo1, 100+comboWindow+1, comboWindow)
	if combo2.Count != 1 {
		t.Errorf("combo count after window lapse = %d, want reset to 1", combo2.Count)
	}
	if result2.ComboCount != 1 {
		t.Errorf("result2.ComboCount = %d, want 1", result2.ComboCount)
	}
}

func TestApplyHits_MultiHitComboSequence_EndingInALethalOverkillHit(t *testing.T) {
	state := newTestState(t, 25, 25)
	events := []hitdetect.HitEvent{{Attacker: match.SideP1, Defender: match.SideP2}}
	comboWindow := 60
	combo := ComboState{}

	state, combo, r1 := ApplyHits(state, events, match.SideP1, hitDefWithDamage("10"), combo, 0, comboWindow)
	if r1.HealthAfter != 15 || combo.Count != 1 {
		t.Fatalf("hit 1: health=%d combo=%d, want 15/1", r1.HealthAfter, combo.Count)
	}

	state, combo, r2 := ApplyHits(state, events, match.SideP1, hitDefWithDamage("10"), combo, 10, comboWindow)
	if r2.HealthAfter != 5 || combo.Count != 2 {
		t.Fatalf("hit 2: health=%d combo=%d, want 5/2", r2.HealthAfter, combo.Count)
	}

	_, combo, r3 := ApplyHits(state, events, match.SideP1, hitDefWithDamage("999"), combo, 20, comboWindow)
	if r3.HealthAfter != 0 {
		t.Errorf("hit 3 (overkill): health = %d, want floored at 0", r3.HealthAfter)
	}
	if combo.Count != 3 {
		t.Errorf("hit 3: combo count = %d, want 3", combo.Count)
	}
}

func TestApplyHits_AcceptsDamageParamsInsteadOfFullController(t *testing.T) {
	state := newTestState(t, 1000, 1000)
	events := []hitdetect.HitEvent{
		{Attacker: match.SideP1, Defender: match.SideP2},
	}

	newState, _, result := ApplyHits(
		state, events, match.SideP1, DamageParams{Damage: "30"}, ComboState{}, 1, 60,
	)

	if got := newState.Fighter(match.SideP2).Health; got != 970 {
		t.Errorf("defender health = %d, want 970", got)
	}
	if result.Damage != 30 {
		t.Errorf("result.Damage = %d, want 30", result.Damage)
	}
}

func TestResetCombo_ReturnsAnInactiveZeroState(t *testing.T) {
	combo := ResetCombo()

	if combo.Active {
		t.Error("expected a reset ComboState to be inactive")
	}
	if combo.Count != 0 {
		t.Errorf("expected a reset ComboState to have Count 0, got %d", combo.Count)
	}
}

func TestApplyHits_StartsAFreshComboAfterAnExplicitReset(t *testing.T) {
	state := newTestState(t, 1000, 1000)
	events := []hitdetect.HitEvent{{Attacker: match.SideP1, Defender: match.SideP2}}

	_, combo, _ := ApplyHits(state, events, match.SideP1, hitDefWithDamage("10"), ComboState{Count: 5, LastHitTick: 0, Active: true}, 1, 60)
	if combo.Count != 6 {
		t.Fatalf("sanity: expected combo to continue to 6, got %d", combo.Count)
	}

	reset := ResetCombo()
	_, combo2, _ := ApplyHits(state, events, match.SideP1, hitDefWithDamage("10"), reset, 2, 60)
	if combo2.Count != 1 {
		t.Errorf("combo count after reset + new hit = %d, want 1", combo2.Count)
	}
}

func TestApplyHits_AllocatesNothing_WhenNoEventMatchesTheAttacker(t *testing.T) {
	state := newTestState(t, 1000, 1000)
	events := make([]hitdetect.HitEvent, 16)
	for i := range events {
		events[i] = hitdetect.HitEvent{Attacker: match.SideP2, Defender: match.SideP1}
	}
	hitDef := hitDefWithDamage("10")
	combo := ComboState{}

	allocs := testing.AllocsPerRun(100, func() {
		ApplyHits(state, events, match.SideP1, hitDef, combo, 1, 60)
	})

	if allocs != 0 {
		t.Errorf("expected 0 allocations when no event matches the attacker, got %v", allocs)
	}
}

func TestApplyHits_AllocatesNothing_WhenAHitLands(t *testing.T) {
	state := newTestState(t, 1000, 1000)
	events := make([]hitdetect.HitEvent, 16)
	for i := range events {
		events[i] = hitdetect.HitEvent{Attacker: match.SideP1, Defender: match.SideP2}
	}
	hitDef := hitDefWithDamage("10")
	combo := ComboState{}

	allocs := testing.AllocsPerRun(100, func() {
		ApplyHits(state, events, match.SideP1, hitDef, combo, 1, 60)
	})

	if allocs != 0 {
		t.Errorf("expected 0 allocations when a hit lands, got %v", allocs)
	}
}

func TestParseHitDef_ReadsTheRealKfmHitDefValues(t *testing.T) {
	// Values of kfm.cns [State 200, 1] (Ikemen GO), keys lower-cased as
	// character/cns stores them.
	got := ParseHitDef(map[string]string{
		"pausetime":        "8, 8",
		"ground.hittime":   "11",
		"ground.slidetime": "5",
		"air.hittime":      "15",
		"animtype":         "Light",
		"ground.type":      "High",
		"air.type":         "High",
	})
	want := HitDefParams{PauseTime: [2]int{8, 8}, GroundHitTime: 11, GroundSlideTime: 5, AirHitTime: 15, AnimType: 0, GroundType: 1, AirType: 1,
		FallAnimType: 3, YAccel: 0.35, FallYVelocity: -4.5, FallEnvShakeFreq: 60, FallEnvShakeAmpl: -4, FallKill: true, DownRecoverTime: -1, FallRecover: true, FallRecoverTime: 4}
	if got != want {
		t.Errorf("ParseHitDef = %+v, want %+v", got, want)
	}
}

func TestParseHitDef_FallsBackToTheDocumentedDefaults_WhenParametersAreMissing(t *testing.T) {
	got := ParseHitDef(nil)
	want := HitDefParams{PauseTime: [2]int{0, 0}, GroundHitTime: 0, GroundSlideTime: 0, AirHitTime: 20, AnimType: 0, GroundType: 1, AirType: 1,
		FallAnimType: 3, YAccel: 0.35, FallYVelocity: -4.5, FallEnvShakeFreq: 60, FallEnvShakeAmpl: -4, FallKill: true, DownRecoverTime: -1, FallRecover: true, FallRecoverTime: 4}
	if got != want {
		t.Errorf("ParseHitDef(nil) = %+v, want %+v", got, want)
	}
}

func TestParseHitDef_AirTypeDefaultsToTheGroundType(t *testing.T) {
	got := ParseHitDef(map[string]string{"ground.type": "Low"})
	if got.GroundType != 2 || got.AirType != 2 {
		t.Errorf("ground.type Low: GroundType=%d AirType=%d, want 2 and 2", got.GroundType, got.AirType)
	}
	got = ParseHitDef(map[string]string{"ground.type": "Low", "air.type": "Trip"})
	if got.GroundType != 2 || got.AirType != 3 {
		t.Errorf("ground.type Low, air.type Trip: GroundType=%d AirType=%d, want 2 and 3", got.GroundType, got.AirType)
	}
}

func TestParseHitDef_MapsEveryAnimTypeAndGroundTypeName(t *testing.T) {
	anim := map[string]int{"light": 0, "Medium": 1, "HARD": 2, "Heavy": 2, "back": 3, "Up": 4, "DiagUp": 5}
	for name, want := range anim {
		if got := ParseHitDef(map[string]string{"animtype": name}).AnimType; got != want {
			t.Errorf("animtype %q = %d, want %d", name, got, want)
		}
	}
	ground := map[string]int{"None": 0, "high": 1, "LOW": 2, "Trip": 3}
	for name, want := range ground {
		if got := ParseHitDef(map[string]string{"ground.type": name}).GroundType; got != want {
			t.Errorf("ground.type %q = %d, want %d", name, got, want)
		}
	}
}

func TestParseHitDef_ExpressionValuesFallBackToTheirDefault(t *testing.T) {
	got := ParseHitDef(map[string]string{
		"pausetime":      "var(0), 3",
		"ground.hittime": "5 + 6",
		"air.hittime":    "Time",
		"animtype":       "Zzz",
	})
	// The literal second pausetime value survives; each non-literal one is
	// replaced by its own default, independently.
	want := HitDefParams{PauseTime: [2]int{0, 3}, GroundHitTime: 0, AirHitTime: 20, AnimType: 0, GroundType: 1, AirType: 1,
		FallAnimType: 3, YAccel: 0.35, FallYVelocity: -4.5, FallEnvShakeFreq: 60, FallEnvShakeAmpl: -4, FallKill: true, DownRecoverTime: -1, FallRecover: true, FallRecoverTime: 4}
	if got != want {
		t.Errorf("ParseHitDef = %+v, want %+v", got, want)
	}
}

func TestParseHitDef_ASinglePauseTimeValueLeavesTheDefenderShakeAtItsDefault(t *testing.T) {
	got := ParseHitDef(map[string]string{"pausetime": "7"})
	if got.PauseTime != [2]int{7, 0} {
		t.Errorf("pausetime = 7: PauseTime = %v, want [7 0]", got.PauseTime)
	}
}

func TestParseHitDef_DoesNotAllocate(t *testing.T) {
	params := map[string]string{"pausetime": "8, 8", "ground.hittime": "11", "animtype": "Light", "ground.type": "High",
		"ground.velocity": "-2.5, -7", "yaccel": ".5", "fall": "1", "fall.yvelocity": "-5.5", "fall.recovertime": "7", "id": "4"}
	allocs := testing.AllocsPerRun(100, func() { _ = ParseHitDef(params) })
	if allocs != 0 {
		t.Errorf("ParseHitDef allocated %v times per call, want 0", allocs)
	}
}

func TestParseHitDef_DecidesAnimTypeAndTypesFromTheFirstNonSpaceLetter_CaseInsensitive(t *testing.T) {
	anim := map[string]int{
		"L": 0, "Med": 1, "H": 2, "Heavy": 2, "  hard": 2, "b": 3, "Up": 4, "U": 4, "D": 5, "DiagUp": 5, "diagup": 5,
	}
	for name, want := range anim {
		if got := ParseHitDef(map[string]string{"animtype": name}).AnimType; got != want {
			t.Errorf("animtype %q = %d, want %d", name, got, want)
		}
	}
	if got := ParseHitDef(map[string]string{"animtype": "lo"}).AnimType; got != 0 {
		t.Errorf("animtype lo = %d, want 0 (Light)", got)
	}
	for _, name := range []string{"", "  "} {
		if got := ParseHitDef(map[string]string{"animtype": name}).AnimType; got != 0 {
			t.Errorf("animtype %q = %d, want 0 (default Light)", name, got)
		}
	}
	if got := ParseHitDef(map[string]string{"animtype": "xyz"}).AnimType; got != 0 {
		t.Errorf("animtype xyz = %d, want 0 (unknown letter -> default Light)", got)
	}
	ground := map[string]int{"N": 0, "h": 1, "lo": 2, " T": 3, "trip": 3}
	for name, want := range ground {
		if got := ParseHitDef(map[string]string{"ground.type": name}).GroundType; got != want {
			t.Errorf("ground.type %q = %d, want %d", name, got, want)
		}
	}
	if got := ParseHitDef(map[string]string{"ground.type": "xyz", "air.type": "q"}); got.GroundType != 1 || got.AirType != 1 {
		t.Errorf("unknown letters: GroundType=%d AirType=%d, want defaults 1 and 1", got.GroundType, got.AirType)
	}
}

func TestParseHitDef_ClampsNegativePauseTimesToZero(t *testing.T) {
	cases := map[string][2]int{
		"0, -1":  {0, 0},
		"-3, 5":  {0, 5},
		"-2, -9": {0, 0},
		"4, 6":   {4, 6},
	}
	for raw, want := range cases {
		if got := ParseHitDef(map[string]string{"pausetime": raw}).PauseTime; got != want {
			t.Errorf("pausetime %q = %v, want %v", raw, got, want)
		}
	}
}

func TestParseHitDef_ReadsIntegerValuesTheWayTheTableSays(t *testing.T) {
	cases := []struct {
		raw  string
		want int
	}{
		{"-1", -1},
		{"+5", 5},
		{" 7 ", 7},
		{"123456789", 123456789},
		{"1234567890", 20}, // 10 digits: not a sane tick count -> default (air.hittime 20)
		{"", 20},
		{"+", 20},
		{"1x", 20},
	}
	for _, c := range cases {
		got := ParseHitDef(map[string]string{"air.hittime": c.raw}).AirHitTime
		if got != c.want {
			t.Errorf("air.hittime %q = %d, want %d", c.raw, got, c.want)
		}
	}
}

func TestApplyHits_TwoValueDamage_AppliesHitDamageAndKeepsGuardDamage(t *testing.T) {
	cases := []struct {
		raw        string
		wantDamage int
		wantGuard  int
	}{
		{"23, 0", 23, 0},
		{"23,5", 23, 5},
		{" 40 , 12 ", 40, 12},
		{"15", 15, 0},
		{"15,", 15, 0},
		{"", 0, 0},
		{"x, 7", 0, 7},
	}
	for _, c := range cases {
		state := newTestState(t, 1000, 1000)
		events := []hitdetect.HitEvent{{Attacker: match.SideP1, Defender: match.SideP2}}

		newState, _, result := ApplyHits(
			state, events, match.SideP1, hitDefWithDamage(c.raw), ComboState{}, 1, 60,
		)

		if result.Damage != c.wantDamage || result.GuardDamage != c.wantGuard {
			t.Errorf("damage %q: Damage/GuardDamage = %d/%d, want %d/%d", c.raw, result.Damage, result.GuardDamage, c.wantDamage, c.wantGuard)
		}
		if got := newState.Fighter(match.SideP2).Health; got != 1000-c.wantDamage {
			t.Errorf("damage %q: defender health = %d, want %d", c.raw, got, 1000-c.wantDamage)
		}
	}
}

// Ikemen GO's HitDef defaults and fields (char.go:730-885 reset, 11680-11810
// where they are copied to the defender's get-hit variables).

func TestParseHitDef_ReadsTheFallAndVelocityParameters(t *testing.T) {
	got := ParseHitDef(map[string]string{
		"ground.velocity":  "-2.5, -7",
		"air.velocity":     "-1, -9.25",
		"yaccel":           ".5",
		"fall":             "1",
		"fall.yvelocity":   "-5.5",
		"fall.recover":     "0",
		"fall.recovertime": "7",
		"id":               "4",
	})
	if got.GroundVelocityY != -7 || got.AirVelocityY != -9.25 {
		t.Errorf("GroundVelocityY, AirVelocityY = %v, %v, want -7, -9.25", got.GroundVelocityY, got.AirVelocityY)
	}
	if got.YAccel != 0.5 || got.FallYVelocity != -5.5 || got.FallRecoverTime != 7 || got.HitID != 4 {
		t.Errorf("YAccel, FallYVelocity, FallRecoverTime, HitID = %v, %v, %v, %v, want 0.5, -5.5, 7, 4", got.YAccel, got.FallYVelocity, got.FallRecoverTime, got.HitID)
	}
	if !got.GroundFall || got.FallRecover {
		t.Errorf("GroundFall, FallRecover = %v, %v, want true, false", got.GroundFall, got.FallRecover)
	}
}

func TestParseHitDef_AirFallDefaultsToTheGroundFall(t *testing.T) {
	if got := ParseHitDef(map[string]string{"fall": "1"}); !got.GroundFall || !got.AirFall {
		t.Errorf("fall = 1: GroundFall, AirFall = %v, %v, want true, true", got.GroundFall, got.AirFall)
	}
	if got := ParseHitDef(map[string]string{"fall": "1", "air.fall": "0"}); !got.GroundFall || got.AirFall {
		t.Errorf("fall = 1, air.fall = 0: GroundFall, AirFall = %v, %v, want true, false", got.GroundFall, got.AirFall)
	}
	if got := ParseHitDef(map[string]string{"air.fall": "1"}); got.GroundFall || !got.AirFall {
		t.Errorf("air.fall = 1: GroundFall, AirFall = %v, %v, want false, true", got.GroundFall, got.AirFall)
	}
}

func TestParseHitDef_AVelocityWithoutAYComponentLeavesYAtZero(t *testing.T) {
	got := ParseHitDef(map[string]string{"ground.velocity": "-3", "air.velocity": "bad, nope"})
	if got.GroundVelocityY != 0 || got.AirVelocityY != 0 {
		t.Errorf("GroundVelocityY, AirVelocityY = %v, %v, want 0, 0", got.GroundVelocityY, got.AirVelocityY)
	}
}

func TestParseHitDef_NonLiteralFallValuesFallBackToTheirDefaults(t *testing.T) {
	got := ParseHitDef(map[string]string{"yaccel": "Const(movement.yaccel)", "fall.yvelocity": "var(1)", "fall.recovertime": "Time", "fall": "var(0)"})
	if got.YAccel != 0.35 || got.FallYVelocity != -4.5 || got.FallRecoverTime != 4 || got.GroundFall {
		t.Errorf("YAccel, FallYVelocity, FallRecoverTime, GroundFall = %v, %v, %v, %v, want 0.35, -4.5, 4, false", got.YAccel, got.FallYVelocity, got.FallRecoverTime, got.GroundFall)
	}
}

// char.go:958-967: air.animtype defaults to animtype; fall.animtype to
// air.animtype when that is Up or DiagUp, else Back.
func TestParseHitDef_AirAndFallAnimTypesDefaultLikeIkemenGO(t *testing.T) {
	cases := []struct {
		name                      string
		params                    map[string]string
		wantGround, wantAir, want int
	}{
		{"nothing given", nil, 0, 0, 3},
		{"animtype only", map[string]string{"animtype": "Medium"}, 1, 1, 3},
		{"air.animtype Up", map[string]string{"animtype": "Light", "air.animtype": "Up"}, 0, 4, 4},
		{"air.animtype DiagUp", map[string]string{"air.animtype": "DiagUp"}, 0, 5, 5},
		{"explicit fall.animtype", map[string]string{"air.animtype": "Up", "fall.animtype": "Hard"}, 0, 4, 2},
	}
	for _, tc := range cases {
		got := ParseHitDef(tc.params)
		if got.AnimType != tc.wantGround || got.AirAnimType != tc.wantAir || got.FallAnimType != tc.want {
			t.Errorf("%s: AnimType, AirAnimType, FallAnimType = %d, %d, %d, want %d, %d, %d", tc.name, got.AnimType, got.AirAnimType, got.FallAnimType, tc.wantGround, tc.wantAir, tc.want)
		}
	}
}
