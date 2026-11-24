package evaluator

import "testing"

// char.go:13001: the final defence is [Data] defence times DefenceMulSet,
// the SuperPause p2defmul and the fall multipliers, over 100.
func TestFinalDefence_IncludesTheSuperAndFallMultipliers(t *testing.T) {
	c := Context{SuperDefenceMul: 1.5, FallDefenceMul: 2}
	if got := c.FinalDefence(); got != 3 {
		t.Errorf("FinalDefence = %v, want 1 * 1.5 * 2 = 3", got)
	}
	var plain Context
	if got := plain.FinalDefence(); got != 1 {
		t.Errorf("FinalDefence without multipliers = %v, want 1", got)
	}
}
