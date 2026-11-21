package round

import "testing"

// stepUntil steps c with intro and decided until it returns want, and
// reports the steps taken (-1 after limit).
func stepUntil(c *Clock, intro, decided bool, want Event, limit int) int {
	for i := 1; i <= limit; i++ {
		if c.Step(intro, decided, true) == want {
			return i
		}
	}
	return -1
}

// system.go:1908-1923, 2528, 3332-3352: RoundState 0 for start.waittime
// ticks, 1 for ctrl.time more, then the fight starts.
func TestClock_IntroThenFight(t *testing.T) {
	c := NewClock(DefaultTimings)
	if c.State() != 0 {
		t.Fatalf("start state = %d, want 0", c.State())
	}
	for i := 0; i < 29; i++ {
		c.Step(false, false, true)
	}
	if c.State() != 0 {
		t.Errorf("after 29 ticks state = %d, want 0", c.State())
	}
	c.Step(false, false, true)
	if c.State() != 1 {
		t.Errorf("after 30 ticks state = %d, want 1", c.State())
	}
	if n := stepUntil(c, false, false, EventFightStart, 100); n != 31 {
		t.Errorf("fight starts %d ticks into RoundState 1, want 31", n)
	}
	if c.State() != 2 {
		t.Errorf("state = %d, want 2", c.State())
	}
	if e := c.Step(false, false, true); e != EventNone || c.State() != 2 {
		t.Errorf("an undecided fight stays at 2, got %v %d", e, c.State())
	}
}

func TestClock_AnIntroAssertHoldsRoundState1(t *testing.T) {
	c := NewClock(DefaultTimings)
	for i := 0; i < 200; i++ {
		if c.Step(true, false, true) == EventFightStart {
			t.Fatal("the fight started while a fighter asserts intro")
		}
	}
	if c.State() != 1 {
		t.Errorf("state = %d, want 1", c.State())
	}
	if n := stepUntil(c, false, false, EventFightStart, 100); n != 31 {
		t.Errorf("fight starts %d ticks after the intro ends, want 31 (ctrl.time + 1)", n)
	}
}

// Decided: RoundState 3 for over.waittime, then 4; the poses come
// over.wintime into 4, the round ends over.time after.
func TestClock_KOThenPosesThenRoundOver(t *testing.T) {
	c := NewClock(DefaultTimings)
	stepUntil(c, false, false, EventFightStart, 100)
	c.Step(false, true, true)
	if c.State() != 3 {
		t.Fatalf("decided state = %d, want 3", c.State())
	}
	// 45 ticks of RoundState 3, then over.wintime (45) counted from the first
	// tick of RoundState 4: the poses come 90 ticks after the decision, 89
	// after its first tick.
	n := stepUntil(c, false, true, EventPoses, 500)
	if n != 89 {
		t.Errorf("poses %d ticks after the first decided one, want 89", n)
	}
	if c.State() != 4 {
		t.Errorf("state at the poses = %d, want 4", c.State())
	}
	if n := stepUntil(c, false, true, EventRoundOver, 1000); n != 211 {
		t.Errorf("round over %d ticks after the poses, want 211 (over.time)", n)
	}
}

// system.go:3441-3466: RoundState 4 waits for every fighter at rest.
func TestClock_RoundState4WaitsForSettledFighters(t *testing.T) {
	c := NewClock(DefaultTimings)
	stepUntil(c, false, false, EventFightStart, 100)
	for i := 0; i < 300; i++ {
		c.Step(false, true, false)
	}
	if c.State() != 3 {
		t.Fatalf("state = %d, want 3 while a fighter is not settled", c.State())
	}
	c.Step(false, true, true)
	if c.State() != 4 {
		t.Errorf("state = %d, want 4 once settled", c.State())
	}
}

// char.go:9526-9540: the inputs wait from over.waittime after the decision
// until the poses, so a fighter still attacking settles.
func TestClock_InputWaitFromOverWaitTimeToThePoses(t *testing.T) {
	c := NewClock(DefaultTimings)
	stepUntil(c, false, false, EventFightStart, 100)
	if c.InputWait() {
		t.Fatal("inputs wait during the fight")
	}
	for i := 0; i < 44; i++ {
		c.Step(false, true, false)
	}
	if c.InputWait() {
		t.Errorf("inputs wait %d ticks into RoundState 3, want not before over.waittime", 44)
	}
	c.Step(false, true, false)
	if !c.InputWait() {
		t.Error("inputs do not wait at over.waittime")
	}
	stepUntil(c, false, true, EventPoses, 500)
	if c.InputWait() {
		t.Error("inputs still wait after the poses")
	}
}

func TestClock_IntroCountsDownToTheFightThenBelowZero(t *testing.T) {
	c := NewClock(Timings{StartWaitTime: 2, CtrlTime: 3, OverWaitTime: 5, OverWinTime: 5, OverTime: 5})
	if c.Intro() != 6 {
		t.Fatalf("start intro = %d, want start.waittime+ctrl.time+1 = 6", c.Intro())
	}
	for range 6 {
		c.Step(false, false, true)
	}
	if c.Intro() != 0 {
		t.Fatalf("fight intro = %d, want 0", c.Intro())
	}
	c.Step(false, true, true)
	if c.Intro() != -1 {
		t.Errorf("decided intro = %d, want -1", c.Intro())
	}
}
