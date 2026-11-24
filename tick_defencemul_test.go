package engine

import (
	"testing"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/match"
)

// char.go:12949-12953: the first acted tick of the bounce (5100) or of the
// lie down from a trip (5070) multiplies the fall defence by
// (fall.defence_up + 100) / 100, unless nofalldefenceup is asserted.
func TestStepFall_ALandingRaisesTheFallDefence(t *testing.T) {
	ctx := evaluator.Context{}
	ctx.MoveType, ctx.StateNo, ctx.Time = 'H', 5100, 1
	stepFall(&ctx, true)
	if ctx.FallDefenceMul != 1.5 {
		t.Fatalf("fall defence after the first bounce = %v, want 1.5 (fall.defence_up 50)", ctx.FallDefenceMul)
	}
	ctx.StateNo = 5070
	stepFall(&ctx, true)
	if ctx.FallDefenceMul != 2.25 {
		t.Errorf("fall defence after a second landing = %v, want 2.25", ctx.FallDefenceMul)
	}
	asserted := evaluator.Context{}
	asserted.MoveType, asserted.StateNo, asserted.Time = 'H', 5100, 1
	if err := asserted.Asserted.Set("nofalldefenceup", true); err != nil {
		t.Fatal(err)
	}
	stepFall(&asserted, true)
	if asserted.FallDefenceMul > 1 {
		t.Errorf("fall defence with nofalldefenceup = %v, want unchanged", asserted.FallDefenceMul)
	}
}

// char.go:12671-12672: out of a hit state, the fall and super defence
// multipliers go back to 1.
func TestStepFall_OutOfAHitTheDefenceMultipliersReset(t *testing.T) {
	ctx := evaluator.Context{FallDefenceMul: 2.25, SuperDefenceMul: 1.5}
	ctx.MoveType = 'I'
	stepFall(&ctx, true)
	if ctx.FinalDefence() != 1 {
		t.Errorf("final defence after the combo = %v, want 1", ctx.FinalDefence())
	}
}

// superPauseMatch has P1 run a SuperPause on its first tick, with params,
// against a P2 standing in a state of the given move type.
func superPauseMatch(t *testing.T, params map[string]string, moveType cns.MoveType) TickResult {
	t.Helper()
	pauser := counterProgram(ctrlWith("SuperPause", "Time = 0", params))
	state, progs, rts := projMatch(t, pauser, 900)
	progs[1] = FighterProgram{States: map[int]cns.StateDef{0: {Number: 0, Type: cns.StateTypeStanding, MoveType: moveType}},
		Animations: []air.Animation{{Number: 0, Frames: []air.Frame{{Time: -1}}}}}
	rt2, err := progs[1].NewRuntime(state.Fighters[1])
	if err != nil {
		t.Fatal(err)
	}
	rts[1] = rt2
	_, all := runProj(t, state, progs, rts, 2)
	return all[0]
}

// char.go:9430-9440, 12987-12995: a SuperPause multiplies the defence of an
// opponent already being hit by p2defmul (Const super.targetdefencemul, 1.5,
// by default); one not being hit keeps its defence (issue 3814).
func TestTick_ASuperPauseRaisesTheDefenceOfAnOpponentBeingHit(t *testing.T) {
	defence := func(res TickResult) float64 {
		ctx := res.Fighters[match.SideP2].Context
		return ctx.FinalDefence()
	}
	if got := defence(superPauseMatch(t, map[string]string{"time": "10"}, cns.MoveTypeHit)); got != 1.5 {
		t.Errorf("defence of a hit opponent = %v, want 1.5 (super.targetdefencemul)", got)
	}
	if got := defence(superPauseMatch(t, map[string]string{"time": "10", "p2defmul": "2"}, cns.MoveTypeHit)); got != 2 {
		t.Errorf("defence with p2defmul = 2: %v, want 2", got)
	}
	if got := defence(superPauseMatch(t, map[string]string{"time": "10", "p2defmul": "2"}, cns.MoveTypeIdle)); got != 1 {
		t.Errorf("defence of an opponent not being hit = %v, want 1", got)
	}
}
