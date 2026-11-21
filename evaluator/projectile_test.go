package evaluator

import "testing"

// char.go:6306: ProjCancelTime is -1 while no projectile was cancelled.
func TestEvaluate_ProjCancelTimeIsMinusOneWithoutCancel(t *testing.T) {
	v, err := Evaluate("ProjCancelTime(1100)", Context{})
	if err != nil || v.Int() != -1 {
		t.Errorf("ProjCancelTime(1100) = %v, %v; want -1", v, err)
	}
}

// InCustomState (Ikemen GO) reads the fighter's being in another player's
// states, also redirected.
func TestEvaluate_InCustomState(t *testing.T) {
	ctx := Context{InOpponentStates: true}
	ctx.P2 = Opponent{Valid: true, InCustomState: false}
	for expr, want := range map[string]bool{"InCustomState": true, "p2,InCustomState": false} {
		v, err := Evaluate(expr, ctx)
		if err != nil || v.Bool() != want {
			t.Errorf("%s = %v, %v; want %v", expr, v, err, want)
		}
	}
}
