package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// Blanka (SFA)'s throw (blanka_N2.cns, statedefs 800 and 810): its HitDef
// grabs a real kfm into Blanka's state 820 while Blanka goes to 810, where
// BindToTarget (pos = 40,70,head) holds him on the thrown kfm's head:
// 40 + kfm's size.head.pos.x (-5) toward kfm's facing, 70 + its
// size.head.pos.y (-90) below it; his bites' HitAdd add to kfm's combo.
func TestE2E_BlankaBindsHimselfToTheThrownKfm(t *testing.T) {
	path := corpusChar(t, "Street Fighter/Blanka (SFA)/Blanka.def")
	blanka, _, err := LoadFighterProgram(loadChar(t, path), path, referenceDir)
	if err != nil {
		t.Fatal(err)
	}
	kfm := loadReferenceProgram(t, kfmCnsDef)
	m := newE2EMatchAt(t, blanka, 800)
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: e2eP1X + 30}, Facing: match.FacingLeft, Health: 1000}
	rt2, err := kfm.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = kfm, rt2, p2
	bound, maxHits := 0, 0
	for i := 0; i < 120; i++ {
		if err := m.step(input.TickInput{}); err != nil {
			t.Fatal(err)
		}
		b, k := m.state.Fighters[match.SideP1], m.state.Fighters[match.SideP2]
		maxHits = max(maxHits, m.runtimes[match.SideP2].Context.GetHit.HitCount)
		// From 810's second tick: kfm, ticking after Blanka, moves in the
		// tick Blanka enters it and the one after.
		if b.StateNo != 810 || k.StateNo != 820 || m.runtimes[match.SideP1].Context.Time < 2 {
			continue
		}
		bound++
		wantX := k.Position.X + (40-5)*facingSign(k.Facing)
		wantY := k.Position.Y - (70 - 90)
		if b.Position.X != wantX || b.Position.Y != wantY {
			t.Fatalf("tick %d: Blanka at %v, kfm at %v facing %v; want Blanka at %v,%v", i, b.Position, k.Position, k.Facing, wantX, wantY)
		}
	}
	if bound == 0 {
		t.Fatal("Blanka never held kfm in 810 / 820")
	}
	// 811's HitAdd (value 1, each bite) adds to kfm's combo (char.go:7814).
	if maxHits < 2 {
		t.Errorf("kfm's hit count peaked at %d, want Blanka's HitAdd to add to it", maxHits)
	}
}
