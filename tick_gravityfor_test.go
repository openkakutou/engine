package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/evaluator"
)

// A fighter in physics A falls by its own movement.yaccel; any other physics
// gets no automatic gravity (Ikemen GO char.go posUpdate).
func TestGravityFor_ReadsTheFightersOwnYAccelInPhysicsA(t *testing.T) {
	for _, c := range []struct {
		physics byte
		want    float64
	}{
		{'A', 0.44},
		{'S', 0},
		{'C', 0},
		{'N', 0},
	} {
		ctx := evaluator.Context{Physics: c.physics, YAccel: 0.44}
		if got := gravityFor(&ctx); got != c.want {
			t.Errorf("physics %c: gravityFor = %v, want %v", c.physics, got, c.want)
		}
	}
}
