package combat

import (
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/hitdetect"
	"github.com/openkakutou/engine/match"
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

func hitDefWithDamage(damage string) cns.Controller {
	return cns.Controller{
		Type:       "HitDef",
		Parameters: map[string]string{"damage": damage},
	}
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
	noDamageParam := cns.Controller{Type: "HitDef", Parameters: map[string]string{}}

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
