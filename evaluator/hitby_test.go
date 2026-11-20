package evaluator

import "testing"

func attr(t *testing.T, raw string, hitdef bool) int32 {
	t.Helper()
	a, err := ParseAttr(raw, hitdef)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// char.go:10969-11042 checkHitBySlot / checkHitByAllSlots: an active NotHitBy
// slot rejects the attributes it names; an active HitBy slot accepts only
// them; non-stacked slots must all accept, stacked ones need any.
func TestHittableBy(t *testing.T) {
	standNormal := attr(t, "S, NA", true)
	airSpecial := attr(t, "A, SA", true)
	var slots [8]HitBySlot
	if !HittableBy(slots, standNormal) {
		t.Error("no active slot: hittable")
	}
	slots[0] = HitBySlot{Attr: attr(t, "SCA", false), Time: 1, Not: true}
	if HittableBy(slots, standNormal) {
		t.Error("NotHitBy SCA: a standing attack must miss")
	}
	slots[0] = HitBySlot{Attr: attr(t, "S, SA", false), Time: 1, Not: true}
	if HittableBy(slots, standNormal) {
		t.Error("NotHitBy S, SA rejects a standing normal too: the state type and the class are checked one by one, any match rejects")
	}
	slots[0] = HitBySlot{Attr: attr(t, "A, SA", false), Time: 1, Not: true}
	if !HittableBy(slots, standNormal) {
		t.Error("NotHitBy A, SA shares nothing with a standing normal: it hits")
	}
	slots[0] = HitBySlot{Attr: attr(t, "C, NA", false), Time: 1}
	if HittableBy(slots, standNormal) || !HittableBy(slots, attr(t, "C, NA", true)) {
		t.Error("HitBy C, NA: only a crouching normal hits")
	}
	slots[0].Time = 0
	if !HittableBy(slots, standNormal) {
		t.Error("an expired slot no longer filters")
	}
	slots[0] = HitBySlot{Attr: attr(t, "S, NA", false), Time: 1, Stack: true}
	slots[1] = HitBySlot{Attr: attr(t, "A, SA", false), Time: 1, Stack: true}
	if !HittableBy(slots, standNormal) || !HittableBy(slots, airSpecial) || HittableBy(slots, attr(t, "C, HA", true)) {
		t.Error("stacked slots: any one accepting is enough")
	}
}
