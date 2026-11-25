package engine

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// These scenarios run in CI on the committed reference kfm, given a few
// states and one animation of the test's own, until the reference
// characters of backlog item 053 cover these features. The corpus scenarios
// (e2e_helper_test.go, e2e_bindtotarget_test.go, e2e_trans_test.go,
// e2e_projtrade_test.go) stay local.

// strikeAnim is a one-frame animation hitting and hurtable 10 to 50 units in
// front, 40 to 60 units up.
const strikeAnim = 9001

// kfmWith is the reference kfm plus the .cns states of src and strikeAnim.
func kfmWith(t *testing.T, src string) FighterProgram {
	t.Helper()
	prog := loadReferenceProgram(t, kfmCnsDef)
	parsed, err := cns.Parse(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	states := maps.Clone(prog.States)
	for _, s := range parsed {
		if s.Written == nil {
			s.Written = map[string]string{}
		}
		states[s.Number] = s
	}
	prog.States = states
	box := air.ClsnBox{Left: 10, Top: -60, Right: 50, Bottom: -40}
	strike := air.Animation{Number: strikeAnim, Frames: []air.Frame{{Time: -1, Clsn1: []air.ClsnBox{box}, Clsn2: []air.ClsnBox{box}}}}
	prog.Animations = append(slices.Clone(prog.Animations), strike)
	index := maps.Clone(prog.animationTable())
	index[strikeAnim] = strike
	prog.animIndex = index
	return prog
}

// facing kfm as P2 at x, idle, with the reference kfm or other.
func withP2(t *testing.T, m *e2eMatch, prog FighterProgram, x float64, stateNo int) {
	t.Helper()
	p2 := match.FighterState{Side: match.SideP2, Position: match.Position{X: x}, Facing: match.FacingLeft, Health: 1000, StateNo: stateNo}
	rt, err := prog.NewRuntime(p2)
	if err != nil {
		t.Fatal(err)
	}
	m.programs[match.SideP2], m.runtimes[match.SideP2], m.state.Fighters[match.SideP2] = prog, rt, p2
}

// State 3000 spawns helper 3001, which flies forward with a HitDef and
// destroys itself once it hits: the helper appears, hits kfm for its 40
// damage and is gone.
func TestE2E_ReferenceHelperFliesAndHits(t *testing.T) {
	prog := kfmWith(t, `
[Statedef 3000]
type = S
movetype = I
physics = S
anim = 0
ctrl = 0
[State 3000, spawn]
type = Helper
trigger1 = Time = 0
name = "ball"
id = 3001
stateno = 3001
pos = 20, 0
postype = p1
[State 3000, done]
type = ChangeState
trigger1 = Time >= 90
value = 0
ctrl = 1

[Statedef 3001]
type = A
movetype = A
physics = N
anim = 9001
velset = 4, 0
[State 3001, hit]
type = HitDef
trigger1 = 1
attr = S, NP
damage = 40
pausetime = 0, 0
[State 3001, gone]
type = DestroySelf
trigger1 = MoveContact
`)
	m := newE2EMatchAt(t, prog, 3000)
	withP2(t, m, loadReferenceProgram(t, kfmCnsDef), e2eP1X+150, 0)
	seen := false
	for range 90 {
		if err := m.step(input.TickInput{}); err != nil {
			t.Fatal(err)
		}
		for _, h := range m.runtimes[match.SideP1].Helpers {
			seen = seen || h.Context.HelperID == 3001
		}
	}
	if !seen {
		t.Fatal("helper 3001 never appeared")
	}
	if h := m.state.Fighters[match.SideP2].Health; h != 960 {
		t.Errorf("kfm's life = %d, want 960 (the helper's 40)", h)
	}
	if n := len(m.runtimes[match.SideP1].Helpers); n != 0 {
		t.Errorf("%d helpers left, want the helper destroyed after its hit", n)
	}
}

// State 3100 hits kfm, then BindToTarget (pos = 50, 0) holds P1 50 units in
// front of its target, along the target's facing, at the target's position
// of the start of the tick (P1 acts first).
func TestE2E_ReferenceBindToTargetHoldsTheAttackerOnItsTarget(t *testing.T) {
	prog := kfmWith(t, `
[Statedef 3100]
type = S
movetype = A
physics = S
anim = 9001
ctrl = 0
[State 3100, hit]
type = HitDef
trigger1 = Time = 0
attr = S, NA
damage = 10
pausetime = 0, 0
[State 3100, bind]
type = BindToTarget
trigger1 = NumTarget > 0
pos = 50, 0
`)
	m := newE2EMatchAt(t, prog, 3100)
	withP2(t, m, loadReferenceProgram(t, kfmCnsDef), e2eP1X+40, 0)
	bound := 0
	for i := range 30 {
		// P1 binds before kfm ticks: it follows kfm as kfm stood when the
		// tick began.
		d := m.state.Fighters[match.SideP2]
		if err := m.step(input.TickInput{}); err != nil {
			t.Fatal(err)
		}
		n, err := evaluator.Evaluate("NumTarget", m.runtimes[match.SideP1].Context)
		if err != nil {
			t.Fatal(err)
		}
		if n.Int() == 0 {
			continue
		}
		bound++
		a := m.state.Fighters[match.SideP1]
		if want := d.Position.X + 50*facingSign(d.Facing); a.Position.X != want || a.Position.Y != d.Position.Y {
			t.Fatalf("tick %d: P1 at %v, kfm at %v facing %v; want P1 at %v,%v", i, a.Position, d.Position, d.Facing, want, d.Position.Y)
		}
	}
	if bound == 0 {
		t.Fatal("P1 never had kfm as its target")
	}
}

// State 3200's Trans (addalpha, alpha = 256,256) draws the fighter addalpha,
// its alpha clamped to 255 (paramTrans).
func TestE2E_ReferenceTransHoldsInItsState(t *testing.T) {
	prog := kfmWith(t, `
[Statedef 3200]
type = S
physics = S
anim = 0
[State 3200, trans]
type = Trans
trigger1 = 1
trans = addalpha
alpha = 256, 256
`)
	m := newE2EMatchAt(t, prog, 3200)
	if err := m.step(input.TickInput{}); err != nil {
		t.Fatal(err)
	}
	c := m.runtimes[match.SideP1].Context
	if c.StateNo != 3200 || c.Trans != "addalpha" || c.TransAlpha != [2]int{255, 255} {
		t.Fatalf("state %d trans %q alpha %v, want 3200 addalpha 255,255", c.StateNo, c.Trans, c.TransAlpha)
	}
}

// Both fighters throw state 3300's projectile at each other: the projectiles
// meet and trade (char.go:2949-2966, 3080-3180), cancelled before reaching
// anyone.
func TestE2E_ReferenceProjectilesCancelEachOther(t *testing.T) {
	prog := kfmWith(t, `
[Statedef 3300]
type = S
movetype = A
physics = S
anim = 0
ctrl = 0
[State 3300, throw]
type = Projectile
trigger1 = Time = 0
projid = 3300
projanim = 9001
velocity = 4, 0
offset = 0, 0
attr = S, SP
damage = 50
pausetime = 0, 0
[State 3300, done]
type = ChangeState
trigger1 = Time >= 60
value = 0
ctrl = 1
`)
	m := newE2EMatchAt(t, prog, 3300)
	withP2(t, m, prog, e2eP1X+250, 3300)
	thrown, cancelled := false, false
	for range 60 {
		if err := m.step(input.TickInput{}); err != nil {
			t.Fatal(err)
		}
		for side := range 2 {
			for _, p := range m.runtimes[side].Projectiles {
				thrown = true
				cancelled = cancelled || p.Status == ProjectileCancelled
			}
		}
	}
	if !thrown {
		t.Fatal("no projectile was thrown")
	}
	if !cancelled {
		t.Fatal("the projectiles never traded")
	}
	if m.state.Fighters[0].Health != 1000 || m.state.Fighters[1].Health != 1000 {
		t.Errorf("health %d / %d, want both untouched: the projectiles cancelled before reaching anyone", m.state.Fighters[0].Health, m.state.Fighters[1].Health)
	}
}
