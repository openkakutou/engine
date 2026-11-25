package engine

import (
	"testing"

	"github.com/openkakutou/character/air"
)

// animationLength is Ikemen GO's totaltime (anim.go:440-460): -1 only when
// the last frame holds forever; a -1 frame before it restarts the count
// (its -1 included), the frames after it adding up.
func TestAnimationLength_IsIkemenGOsTotalTime(t *testing.T) {
	frames := func(times ...int) air.Animation {
		a := air.Animation{}
		for _, tm := range times {
			a.Frames = append(a.Frames, air.Frame{Time: tm})
		}
		return a
	}
	for _, c := range []struct {
		name string
		anim air.Animation
		want int
	}{
		{"timed frames", frames(3, 4, 5), 12},
		{"last frame forever", frames(3, -1), -1},
		{"forever frame before the end", frames(5, -1, 3), 2},
		{"no frame", frames(), 0},
	} {
		if got := animationLength(c.anim); got != c.want {
			t.Errorf("%s: animationLength = %d, want %d", c.name, got, c.want)
		}
	}
}
