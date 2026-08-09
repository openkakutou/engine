package match

import "testing"

func TestFighterState_ZeroValue_HasUsableDefaultFields(t *testing.T) {
	var fs FighterState

	if fs.Position != (Position{X: 0, Y: 0}) {
		t.Errorf("zero-value FighterState.Position = %+v, want {0 0}", fs.Position)
	}
	if fs.Velocity != (Velocity{X: 0, Y: 0}) {
		t.Errorf("zero-value FighterState.Velocity = %+v, want {0 0}", fs.Velocity)
	}
	if fs.Facing != FacingRight {
		t.Errorf("zero-value FighterState.Facing = %v, want FacingRight", fs.Facing)
	}
	if fs.StateNo != 0 {
		t.Errorf("zero-value FighterState.StateNo = %d, want 0", fs.StateNo)
	}
	if fs.Health != 0 {
		t.Errorf("zero-value FighterState.Health = %d, want 0", fs.Health)
	}
	if fs.Side != SideP1 {
		t.Errorf("zero-value FighterState.Side = %v, want SideP1", fs.Side)
	}
}

func TestMatchState_ZeroValue_HasTwoUsableFighterSlots(t *testing.T) {
	var ms MatchState

	if ms.Round != 0 {
		t.Errorf("zero-value MatchState.Round = %d, want 0", ms.Round)
	}
	if ms.RoundTimer != 0 {
		t.Errorf("zero-value MatchState.RoundTimer = %d, want 0", ms.RoundTimer)
	}
	if len(ms.Fighters) != 2 {
		t.Fatalf("zero-value MatchState.Fighters has %d slots, want 2", len(ms.Fighters))
	}
	// Reading both slots must not panic and must yield usable zero fighters.
	_ = ms.Fighters[0].Position
	_ = ms.Fighters[1].Health
}

func TestNewMatchState_ReturnsPopulatedState_WithTwoValidFighters(t *testing.T) {
	p1 := FighterState{Side: SideP1, Position: Position{X: -50, Y: 0}, Facing: FacingRight, StateNo: 0, Health: 1000}
	p2 := FighterState{Side: SideP2, Position: Position{X: 50, Y: 0}, Facing: FacingLeft, StateNo: 0, Health: 1000}

	ms, err := NewMatchState(1, 99*60, p1, p2)
	if err != nil {
		t.Fatalf("NewMatchState returned error: %v", err)
	}
	if ms.Round != 1 {
		t.Errorf("ms.Round = %d, want 1", ms.Round)
	}
	if ms.RoundTimer != 99*60 {
		t.Errorf("ms.RoundTimer = %d, want %d", ms.RoundTimer, 99*60)
	}
	if got := ms.Fighter(SideP1); got != p1 {
		t.Errorf("ms.Fighter(SideP1) = %+v, want %+v", got, p1)
	}
	if got := ms.Fighter(SideP2); got != p2 {
		t.Errorf("ms.Fighter(SideP2) = %+v, want %+v", got, p2)
	}
}

func TestNewMatchState_ReturnsError_WhenFighterCountIsNotTwo(t *testing.T) {
	only := FighterState{Side: SideP1}

	if _, err := NewMatchState(1, 100, only); err == nil {
		t.Error("NewMatchState with 1 fighter: want error, got nil")
	}

	three := []FighterState{
		{Side: SideP1},
		{Side: SideP2},
		{Side: SideP1},
	}
	if _, err := NewMatchState(1, 100, three...); err == nil {
		t.Error("NewMatchState with 3 fighters: want error, got nil")
	}
}

func TestNewMatchState_ReturnsError_WhenBothFightersShareTheSameSide(t *testing.T) {
	p1 := FighterState{Side: SideP1}
	duplicate := FighterState{Side: SideP1}

	if _, err := NewMatchState(1, 100, p1, duplicate); err == nil {
		t.Error("NewMatchState with two SideP1 fighters: want error, got nil")
	}
}

func TestMatchState_Fighter_ReturnsZeroValueFighter_OnZeroValueMatchState(t *testing.T) {
	var ms MatchState

	// A zero-value MatchState was never populated by NewMatchState, so both
	// slots hold the FighterState zero value regardless of which Side is
	// requested — reading it must not panic.
	if got := ms.Fighter(SideP2); got != (FighterState{}) {
		t.Errorf("ms.Fighter(SideP2) on zero-value MatchState = %+v, want zero FighterState{}", got)
	}
}
