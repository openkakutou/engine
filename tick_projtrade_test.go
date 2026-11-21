package engine

import (
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/engine/v2/match"
)

// tradeMatch has both players throw a projectile at each other on tick 1,
// each with hit and hurt boxes, P2's with p2params.
func tradeMatch(t *testing.T, p1params, p2params map[string]string) (*match.MatchState, [2]FighterProgram, [2]FighterRuntime) {
	t.Helper()
	thrower := func(params map[string]string) FighterProgram {
		p := casterProgram(params)
		p.Animations[1] = air.Animation{Number: 100, Frames: []air.Frame{{Time: -1, Clsn1: projBox, Clsn2: projBox}}}
		return p
	}
	state, progs, rts := projMatch(t, thrower(p1params), 200)
	progs[1] = thrower(p2params)
	rt, err := progs[1].NewRuntime(state.Fighters[1])
	if err != nil {
		t.Fatal(err)
	}
	rts[1] = rt
	return state, progs, rts
}

// char.go:2949-2966: two projectiles of equal priority (1) and hits (1)
// cancel each other; each owner's ProjCancelTime starts counting.
func TestTick_ProjectilesOfEqualPriorityCancelEachOther(t *testing.T) {
	cancelTime := map[string]string{"projid": "5"}
	state, progs, rts := tradeMatch(t, cancelTime, cancelTime)
	_, all := runProj(t, state, progs, rts, 40)
	cancelled := [2]bool{}
	for _, r := range all {
		for side := range 2 {
			for _, p := range r.Fighters[side].Projectiles {
				if p.Status == ProjectileCancelled {
					cancelled[side] = true
				}
			}
		}
	}
	if !cancelled[0] || !cancelled[1] {
		t.Fatalf("cancelled = %v, want both", cancelled)
	}
	for side := range 2 {
		c := all[len(all)-1].Fighters[side].Context
		if len(c.ProjContacts) == 0 || c.ProjContacts[0].Cancel == 0 {
			t.Errorf("side %d contacts %+v, want a cancel recorded", side, c.ProjContacts)
		}
	}
}

// A projpriority above the other's wins the trade: it loses a priority point
// and flies on, the other is cancelled.
func TestTick_HigherPriorityProjectileWinsTheTrade(t *testing.T) {
	state, progs, rts := tradeMatch(t, map[string]string{"projpriority": "2"}, nil)
	_, all := runProj(t, state, progs, rts, 40)
	var p1Cancelled, p2Cancelled bool
	for _, r := range all {
		for _, p := range r.Fighters[0].Projectiles {
			p1Cancelled = p1Cancelled || p.Status == ProjectileCancelled
		}
		for _, p := range r.Fighters[1].Projectiles {
			p2Cancelled = p2Cancelled || p.Status == ProjectileCancelled
		}
	}
	if p1Cancelled || !p2Cancelled {
		t.Errorf("P1 cancelled %v, P2 cancelled %v; want only P2's", p1Cancelled, p2Cancelled)
	}
}
