package statemachine

import (
	"strings"
	"testing"

	"github.com/openkakutou/engine/v2/evaluator"
)

func withPower(t *testing.T, power string) evaluator.Context {
	t.Helper()
	c, err := evaluator.ParseConstants(strings.NewReader("[Data]\npower = " + power + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	return evaluator.Context{Consts: c}
}

// Ikemen GO clamps every power change to the character's powerMax, its
// [Data] power (setPower, char.go:9019-9029; powerMax char.go:4148).
func TestPowerAdd_IsCappedAtTheCharactersOwnPower(t *testing.T) {
	for _, tc := range []struct {
		power string
		add   string
		want  int
	}{
		{"1000", "1500", 1000},
		{"5000", "4000", 4000},
		{"5000", "6000", 5000},
	} {
		ctx := withPower(t, tc.power)
		run(t, &ctx, "PowerAdd", map[string]string{"value": tc.add})
		if ctx.Power != tc.want {
			t.Errorf("power = %s, PowerAdd %s: got %d, want %d", tc.power, tc.add, ctx.Power, tc.want)
		}
	}
}

func TestPowerMaxTrigger_IsTheSameCap(t *testing.T) {
	ctx := withPower(t, "5000")
	v, err := evaluator.Evaluate("PowerMax", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if v.Int() != 5000 || ctx.PowerMax() != 5000 {
		t.Errorf("PowerMax = %d, ctx.PowerMax() = %d, want 5000", v.Int(), ctx.PowerMax())
	}
	none := evaluator.Context{}
	if none.PowerMax() != DefaultMaxPower {
		t.Errorf("no constants: PowerMax() = %d, want %d", none.PowerMax(), DefaultMaxPower)
	}
}
