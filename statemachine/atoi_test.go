package statemachine

import (
	"math"
	"testing"
)

// ikemenAtoi is Ikemen GO's Atoi (common.go:191-225): the leading signed
// digits, IMax past the int32 range, IErr past it on the negative side.
func TestIkemenAtoi_ReadsLeadingDigitsAndSaturates(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want int32
	}{
		{" 12abc", 12}, {"-7", -7}, {"+3", 3}, {"", 0}, {"x1", 0},
		{"2147483647", math.MaxInt32}, {"-2147483648", math.MinInt32},
		{"99999999999", math.MaxInt32}, {"-99999999999", math.MinInt32},
	} {
		if got := ikemenAtoi(c.raw); got != c.want {
			t.Errorf("ikemenAtoi(%q) = %d, want %d", c.raw, got, c.want)
		}
	}
}

// persistent is 1 when absent or above 128, "once" (MaxInt32) at 0 or below
// (compiler.go:7319-7330), huge values included.
func TestPersistentValue_FollowsIkemenGO(t *testing.T) {
	for _, c := range []struct {
		raw  string
		want int32
	}{
		{"", 1}, {"5", 5}, {"128", 128}, {"129", 1}, {"0", math.MaxInt32},
		{"-3", math.MaxInt32}, {"99999999999", 1}, {"-99999999999", math.MaxInt32},
	} {
		if got := persistentValue(c.raw); got != c.want {
			t.Errorf("persistentValue(%q) = %d, want %d", c.raw, got, c.want)
		}
	}
}
