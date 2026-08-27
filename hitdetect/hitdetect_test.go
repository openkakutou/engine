package hitdetect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/engine/match"
)

// loadFixtureAnimations parses this package's own copy of character/air's
// real sample fixture -- see .vibe/decisions/009 for why a real .air
// fixture is used rather than hand-built ClsnBox literals.
func loadFixtureAnimations(t *testing.T) []air.Animation {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", "sample.air"))
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer f.Close()

	anims, err := air.Parse(f)
	if err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}
	return anims
}

// attackFrame is Action 200's frame index 2: Clsn1=[{-3,-73,15,-1}],
// Clsn2=[{-6,-92,6,0}] (the Clsn2Default inherited by every frame of the
// action) -- a frame with both an attack box and a vulnerable box.
func attackFrame(t *testing.T) air.Frame {
	t.Helper()
	anims := loadFixtureAnimations(t)
	for _, a := range anims {
		if a.Number == 200 {
			return a.Frames[2]
		}
	}
	t.Fatal("fixture is missing Action 200")
	return air.Frame{}
}

// defendFrame is Action 200's frame index 0: Clsn1=[] (no attack box),
// Clsn2=[{-6,-92,6,0}] -- a frame with only a vulnerable box.
func defendFrame(t *testing.T) air.Frame {
	t.Helper()
	anims := loadFixtureAnimations(t)
	for _, a := range anims {
		if a.Number == 200 {
			return a.Frames[0]
		}
	}
	t.Fatal("fixture is missing Action 200")
	return air.Frame{}
}

// blankFrame is Action 201's frame index 0: Clsn1=[], Clsn2=[] -- no
// collision boxes at all, a valid case (e.g. an idle frame).
func blankFrame(t *testing.T) air.Frame {
	t.Helper()
	anims := loadFixtureAnimations(t)
	for _, a := range anims {
		if a.Number == 201 {
			return a.Frames[0]
		}
	}
	t.Fatal("fixture is missing Action 201")
	return air.Frame{}
}

func fighter(side match.Side, x, y float64, facing match.Facing) match.FighterState {
	return match.FighterState{Side: side, Position: match.Position{X: x, Y: y}, Facing: facing}
}

func newState(t *testing.T, p1, p2 match.FighterState) *match.MatchState {
	t.Helper()
	ms, err := match.NewMatchState(1, 0, p1, p2)
	if err != nil {
		t.Fatalf("failed to build match state: %v", err)
	}
	return ms
}

func TestDetect_ReportsHitEvent_WhenAttackerClsn1OverlapsDefenderClsn2(t *testing.T) {
	p1 := fighter(match.SideP1, 0, 0, match.FacingRight)
	p2 := fighter(match.SideP2, 5, 0, match.FacingRight)
	state := newState(t, p1, p2)
	frames := [2]air.Frame{attackFrame(t), defendFrame(t)}

	events := Detect(state, frames)

	if len(events) != 1 {
		t.Fatalf("expected exactly 1 hit event, got %d: %+v", len(events), events)
	}
	got := events[0]
	if got.Attacker != match.SideP1 || got.Defender != match.SideP2 {
		t.Errorf("expected attacker=SideP1 defender=SideP2, got attacker=%v defender=%v", got.Attacker, got.Defender)
	}
	wantAttackerBox := air.ClsnBox{Left: -3, Top: -73, Right: 15, Bottom: -1}
	wantDefenderBox := air.ClsnBox{Left: -6, Top: -92, Right: 6, Bottom: 0}
	if got.AttackerBox != wantAttackerBox {
		t.Errorf("AttackerBox = %+v, want %+v", got.AttackerBox, wantAttackerBox)
	}
	if got.DefenderBox != wantDefenderBox {
		t.Errorf("DefenderBox = %+v, want %+v", got.DefenderBox, wantDefenderBox)
	}
}

func TestDetect_ReportsNoEvent_WhenBoxesDoNotOverlap(t *testing.T) {
	p1 := fighter(match.SideP1, 0, 0, match.FacingRight)
	p2 := fighter(match.SideP2, 200, 0, match.FacingRight)
	state := newState(t, p1, p2)
	frames := [2]air.Frame{attackFrame(t), defendFrame(t)}

	events := Detect(state, frames)

	if len(events) != 0 {
		t.Fatalf("expected no hit events for far-apart fighters, got %d: %+v", len(events), events)
	}
}

func TestDetect_MirrorsAttackerBox_WhenFacingLeft(t *testing.T) {
	// Defender sits to the attacker's left. The attacker's Clsn1 box
	// ([-3,15] on X) only reaches that far once mirrored by FacingLeft
	// (becoming [-15,3]) -- isolating the facing-mirror behavior from
	// plain position overlap.
	p1Right := fighter(match.SideP1, 0, 0, match.FacingRight)
	p1Left := fighter(match.SideP1, 0, 0, match.FacingLeft)
	p2 := fighter(match.SideP2, -10, 0, match.FacingRight)
	frames := [2]air.Frame{attackFrame(t), defendFrame(t)}

	stateFacingRight := newState(t, p1Right, p2)
	if events := Detect(stateFacingRight, frames); len(events) != 0 {
		t.Fatalf("facing right: expected no hit event (box not mirrored toward defender), got %d: %+v", len(events), events)
	}

	stateFacingLeft := newState(t, p1Left, p2)
	events := Detect(stateFacingLeft, frames)
	if len(events) != 1 {
		t.Fatalf("facing left: expected 1 hit event (box mirrored toward defender), got %d: %+v", len(events), events)
	}
	if events[0].Attacker != match.SideP1 || events[0].Defender != match.SideP2 {
		t.Errorf("expected attacker=SideP1 defender=SideP2, got attacker=%v defender=%v", events[0].Attacker, events[0].Defender)
	}
}

func TestDetect_ReportsNoEvent_WhenFrameHasNoClsnBoxes(t *testing.T) {
	p1 := fighter(match.SideP1, 0, 0, match.FacingRight)
	p2 := fighter(match.SideP2, 0, 0, match.FacingRight)
	state := newState(t, p1, p2)
	// Same position, so any non-empty boxes would trivially overlap --
	// the blank frame having zero boxes is what must produce zero events,
	// not distance.
	frames := [2]air.Frame{blankFrame(t), blankFrame(t)}

	events := Detect(state, frames)

	if len(events) != 0 {
		t.Fatalf("expected no hit events for boxless frames, got %d: %+v", len(events), events)
	}
}

func TestDetect_ReportsBothDirections_WhenBothFightersAttackEachOther(t *testing.T) {
	p1 := fighter(match.SideP1, 0, 0, match.FacingRight)
	p2 := fighter(match.SideP2, 5, 0, match.FacingRight)
	state := newState(t, p1, p2)
	// Both fighters use the frame that has both an attack and a
	// vulnerable box, so each should land a hit on the other.
	frames := [2]air.Frame{attackFrame(t), attackFrame(t)}

	events := Detect(state, frames)

	if len(events) != 2 {
		t.Fatalf("expected 2 hit events (one per direction), got %d: %+v", len(events), events)
	}
	sides := map[match.Side]match.Side{}
	for _, e := range events {
		sides[e.Attacker] = e.Defender
	}
	if sides[match.SideP1] != match.SideP2 || sides[match.SideP2] != match.SideP1 {
		t.Errorf("expected one event per direction, got %+v", events)
	}
}

func TestDetect_AllocatesNothing_OnAZeroHitTick(t *testing.T) {
	p1 := fighter(match.SideP1, 0, 0, match.FacingRight)
	p2 := fighter(match.SideP2, 200, 0, match.FacingRight)
	state := newState(t, p1, p2)
	frames := [2]air.Frame{attackFrame(t), defendFrame(t)}

	allocs := testing.AllocsPerRun(100, func() {
		Detect(state, frames)
	})

	if allocs != 0 {
		t.Errorf("expected 0 allocations on a zero-hit tick, got %v", allocs)
	}
}
