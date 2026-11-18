package evaluator

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/openkakutou/character/air"
)

// kfmAnimations parses the real kfm.air into the lookup Context.Anims holds.
func kfmAnimations(t *testing.T) map[int]air.Animation {
	t.Helper()
	f, err := os.Open("../testdata/reference/chars/kfm/kfm.air")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	list, err := air.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	table := make(map[int]air.Animation, len(list))
	for _, a := range list {
		table[a.Number] = a
	}
	return table
}

// Expected values below are worked out by hand from kfm.air and Ikemen GO's
// anim.go (commit 54d4148a):
//
//	Action 200: element times 2,1,4,3,2 -> element starts 0,2,3,7,10, total 12,
//	            loops from element 1.
//	Action 600: element times 3,11,4,-1 -> starts 0,3,14,18, ends held (-1).
//	Action 41:  nine elements, times 7,4,4,4,4,4,4,4,4, "Loopstart" before the
//	            8th: starts 0,7,11,15,19,23,27,31,35, total 39, loop time 8.
//
// AnimElemTime(e) is the time spent since element e started, i.e. curtime
// minus the sum of the earlier elements' durations (anim.go:621-634), where
// curtime follows Action (anim.go:923-968): it counts ticks, and past the
// total wraps to total-looptime+1.
func animCtx(t *testing.T, anim, animTime int) Context {
	t.Helper()
	return Context{Anim: anim, AnimTime: animTime, Anims: kfmAnimations(t)}
}

func TestEval_AnimElemTime_MeasuresFromTheStartOfThatElement(t *testing.T) {
	cases := []struct {
		name           string
		anim, animTime int
		elem           int
		want           float64
	}{
		{"first tick, element 1", 200, 0, 1, 0},
		{"first tick, later element not started", 200, 0, 5, -10},
		{"inside element 3 (starts at 3)", 200, 5, 3, 2},
		{"element 1 is long past", 200, 5, 1, 5},
		{"next element not yet started", 200, 5, 4, -2},
		{"held last element (-1), earlier element", 600, 100, 3, 86},
		{"held last element (-1), itself", 600, 100, 4, 82},
		{"looping animation, second pass", 200, 13, 1, 1},
		// At the very tick the loop wraps curtime still equals the total
		// (anim.go:960-964 wraps on the next Action), so Ikemen GO reads 12.
		{"looping animation, the wrap tick itself", 200, 12, 1, 12},
		// Action 41 loops from its 8th element (start 31): after a pass
		// curtime = 39-8+1 = 32 at tick 40.
		{"loopstart, after the wrap", 41, 40, 8, 1},
		{"loopstart, element after the loop element", 41, 40, 9, -3},
		{"loopstart, an element before the loop", 41, 40, 1, 32},
	}
	for _, tc := range cases {
		ctx := animCtx(t, tc.anim, tc.animTime)
		ctx.Time = 0
		got := mustEval(t, "AnimElemTime("+strconv.Itoa(tc.elem)+")", ctx)
		if got.Kind != KindInt || got.Number != tc.want {
			t.Errorf("%s: AnimElemTime(%d) at anim %d time %d = %v (kind %v), want int %v", tc.name, tc.elem, tc.anim, tc.animTime, got.Number, got.Kind, tc.want)
		}
	}
}

func TestEval_AnimElemTime_ElementOutOfRangeOrNoAnimation_ReadsZeroLikeIkemenUndefined(t *testing.T) {
	// Ikemen GO yields an undefined value (char.go:5573-5578) that reads as 0
	// in a comparison (bytecode.go:1328-1333): not an error.
	for _, expr := range []string{"AnimElemTime(6)", "AnimElemTime(0)", "AnimElemTime(-1)"} {
		got := mustEval(t, expr, animCtx(t, 200, 3))
		if got.Number != 0 {
			t.Errorf("%s on a 5-element animation = %v, want 0", expr, got.Number)
		}
	}
	// 9999 is no animation of kfm.air; a nil table has none either.
	for _, ctx := range []Context{animCtx(t, 9999, 3), {Anim: 200, AnimTime: 3}} {
		if got := mustEval(t, "AnimElemTime(1)", ctx); got.Number != 0 {
			t.Errorf("AnimElemTime(1) with no animation = %v, want 0", got.Number)
		}
	}
}

func TestEval_AnimElemNo_ElementPlayingAtAnOffsetInTime(t *testing.T) {
	cases := []struct {
		name           string
		anim, animTime int
		offset         string
		want           float64
	}{
		{"now", 200, 5, "0", 3},
		{"two ticks ago, element 3 just started", 200, 5, "-2", 3},
		{"three ticks ago, element 2", 200, 5, "-3", 2},
		{"in 2 ticks, element 4 starts", 200, 5, "2", 4},
		{"forward past the end wraps to element 1", 200, 11, "2", 1},
		{"held last element, forward", 600, 100, "50", 4},
		{"held last element, 90 ticks ago", 600, 100, "-90", 2},
		// Before the animation started: Ikemen GO's undefined (char.go:5567).
		{"before the animation began", 600, 100, "-101", 0},
		{"unknown animation", 9999, 100, "0", 0},
	}
	for _, tc := range cases {
		got := mustEval(t, "AnimElemNo("+tc.offset+")", animCtx(t, tc.anim, tc.animTime))
		if got.Kind != KindInt || got.Number != tc.want {
			t.Errorf("%s: AnimElemNo(%s) at anim %d time %d = %v, want %v", tc.name, tc.offset, tc.anim, tc.animTime, got.Number, tc.want)
		}
	}
}

func TestEval_AnimElemEquals_TrueOnlyWhenTheElementTimeComparisonHolds(t *testing.T) {
	cases := []struct {
		expr     string
		animTime int
		want     bool
	}{
		// "AnimElem = n" is AnimElemTime(n) = 0: the first tick of element n
		// (compiler.go:4297-4318).
		{"AnimElem = 3", 3, true},
		{"AnimElem = 3", 4, false},
		{"AnimElem = 3", 2, false},
		{"animelem = 3, >= 2", 5, true},
		{"AnimElem = 3, >= 2", 4, false},
		{"AnimElem = 3, < 0", 2, true},
		{"AnimElem = 3, > 0", 2, false},
		{"AnimElem = 3, <= 1", 4, true},
		{"AnimElem = 3, != 2", 4, true},
		{"AnimElem = 3, 2", 5, true},
		{"AnimElem = 3, -2", 1, true},
		// Parts of a larger trigger.
		{"AnimElem = 3 && Anim = 200", 3, true},
		{"AnimElem = 3, >= 2 && AnimElem = 1, > 4", 5, true},
		{"!(AnimElem = 3)", 3, false},
		// An element the animation lacks never holds (the comparison is
		// skipped on an undefined value, compiler.go OC_jud8).
		{"AnimElem = 6", 0, false},
		{"AnimElem = 6, < 0", 0, false},
	}
	for _, tc := range cases {
		got := mustEval(t, tc.expr, animCtx(t, 200, tc.animTime))
		if got.Bool() != tc.want {
			t.Errorf("%q at AnimTime %d = %v, want %v", tc.expr, tc.animTime, got.Bool(), tc.want)
		}
	}
	if mustEval(t, "AnimElem = 1", animCtx(t, 9999, 0)).Bool() {
		t.Error("AnimElem = 1 on an animation kfm.air lacks held")
	}
}

func TestParse_AnimElemMisuse_IsADescriptiveErrorNotAnUnknownTrigger(t *testing.T) {
	cases := []struct{ expr, wantInError string }{
		{"AnimElem = 0", "greater than 0"},
		{"AnimElem = 3.5", "integer"},
		{"AnimElem != 3", "!="},
		{"AnimElem > 3", "="},
		{"AnimElem = var(0)", "integer"},
		{"AnimElem = 3, >=", "unexpected"},
	}
	for _, tc := range cases {
		_, err := Parse(tc.expr)
		if err == nil || !strings.Contains(err.Error(), tc.wantInError) {
			t.Errorf("Parse(%q) error = %v, want one mentioning %q", tc.expr, err, tc.wantInError)
		}
	}
	_, err := Evaluate("AnimElem", Context{})
	var unknown *UnknownTriggerError
	if err == nil || errors.As(err, &unknown) {
		t.Errorf("bare AnimElem error = %v, want a descriptive non-unknown error", err)
	}
}

func TestEval_AnimExistAndSelfAnimExist_ReadTheAnimationTable(t *testing.T) {
	cases := []struct {
		expr string
		anim int
		want bool
	}{
		{"AnimExist(200)", 0, true},
		{"AnimExist(9999)", 0, false},
		{"SelfAnimExist(41)", 0, true},
		{"SelfAnimExist(44)", 0, false}, // kfm.air has no action 44
		// The expression form common1.cns.zss:151 uses.
		{"SelfAnimExist(anim + 3)", 40, true},  // 43
		{"SelfAnimExist(anim + 3)", 41, false}, // 44
		{"!SelfAnimExist(5051 + 1)", 0, true},  // 5052 is absent, as kfm_zss hits.zss:153 assumes
		{"SelfAnimExist(200.9)", 0, true},      // an int argument, truncated as Ikemen GO's ToI
	}
	for _, tc := range cases {
		ctx := animCtx(t, tc.anim, 0)
		if got := mustEval(t, tc.expr, ctx); got.Bool() != tc.want || got.Kind != KindBool {
			t.Errorf("%s with Anim %d = %v (kind %v), want %v", tc.expr, tc.anim, got.Bool(), got.Kind, tc.want)
		}
	}
	if mustEval(t, "SelfAnimExist(200)", Context{}).Bool() || mustEval(t, "AnimExist(200)", Context{}).Bool() {
		t.Error("an empty animation table reported animation 200 as existing")
	}
}

func TestEval_AnimTriggerWrongArgumentCount_IsAnErrorNotUnknown(t *testing.T) {
	for _, expr := range []string{"AnimElemTime()", "AnimElemNo(1, 2)", "AnimExist()", "SelfAnimExist(1, 2)"} {
		_, err := Evaluate(expr, Context{})
		var unknown *UnknownTriggerError
		if err == nil || errors.As(err, &unknown) || !strings.Contains(err.Error(), "argument") {
			t.Errorf("Evaluate(%q) error = %v, want an argument-count error", expr, err)
		}
	}
}

func TestIsKnownTrigger_AnimationFamily_IsKnown(t *testing.T) {
	for _, tc := range []struct {
		name string
		call bool
	}{{"animelem", false}, {"animelemtime", true}, {"animelemno", true}, {"animexist", true}, {"selfanimexist", true}} {
		if !IsKnownTrigger(tc.name, tc.call) {
			t.Errorf("IsKnownTrigger(%q, %v) = false", tc.name, tc.call)
		}
	}
	names, err := TriggerNames("AnimElem = 3, >= 2 && SelfAnimExist(anim + 3)")
	if err != nil {
		t.Fatal(err)
	}
	want := []TriggerName{{Name: "animelem"}, {Name: "selfanimexist", Call: true}, {Name: "anim"}}
	if len(names) != len(want) {
		t.Fatalf("TriggerNames = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("TriggerNames[%d] = %v, want %v", i, names[i], want[i])
		}
	}
}

func TestElementAt_ReturnsTheElementAndTicksSpentInIt(t *testing.T) {
	anims := kfmAnimations(t)
	cases := []struct {
		anim, animTime, wantElem, wantElapsed int
	}{
		{200, 0, 0, 0},
		{200, 5, 2, 2},
		{200, 12, 0, 0},    // wraps to the first element
		{41, 40, 7, 1},     // wraps to the loopstart element
		{600, 500, 3, 482}, // the held last element keeps counting
	}
	for _, tc := range cases {
		elem, elapsed := ElementAt(anims[tc.anim], tc.animTime)
		if elem != tc.wantElem || elapsed != tc.wantElapsed {
			t.Errorf("ElementAt(anim %d, %d) = %d, %d, want %d, %d", tc.anim, tc.animTime, elem, elapsed, tc.wantElem, tc.wantElapsed)
		}
	}
}

func TestEval_AnimTriggersWarmPath_AllocateNothing(t *testing.T) {
	ctx := animCtx(t, 41, 40)
	expr := "AnimElem = 8, >= 0 && SelfAnimExist(anim + 3) || AnimElemTime(8) > 0 || AnimElemNo(0) = 3 || AnimExist(1)"
	if _, err := EvaluateMut(expr, &ctx); err != nil {
		t.Fatal(err)
	}
	if allocs := testing.AllocsPerRun(100, func() { _, _ = EvaluateMut(expr, &ctx) }); allocs != 0 {
		t.Errorf("allocated %v times per call, want 0", allocs)
	}
}
