package round

// Timings are a round's announcement times, a fight.def's [Round] values
// (Ikemen GO FightScreenRound, fs.go:3298-3312, commit 54d4148a).
type Timings struct {
	StartWaitTime, CtrlTime, OverWaitTime, OverWinTime, OverTime int
}

// DefaultTimings are Ikemen GO's defaults (fs.go:3301-3310).
var DefaultTimings = Timings{StartWaitTime: 30, CtrlTime: 30, OverWaitTime: 45, OverWinTime: 45, OverTime: 210}

// Clock is a round's progress as Ikemen GO keeps it in sys.intro
// (system.go:1908-1923, 2528, 3318-3470): counting down from the round's
// start to 0, the fight, then below 0 once the round is decided.
type Clock struct {
	t     Timings
	intro int
	// winPose counts down to the poses once RoundState 4 is reached.
	winPose int
	posed   bool
}

// NewClock starts a round's clock: intro = start.waittime + ctrl.time + 1
// (system.go:2528).
func NewClock(t Timings) *Clock {
	return &Clock{t: t, intro: t.StartWaitTime + t.CtrlTime + 1, winPose: t.OverWinTime}
}

// State is Ikemen GO's RoundState (system.go:1908-1923): 0 before the
// intros, 1 during them, 2 the fight, 3 decided and 4 once over.waittime has
// passed.
func (c *Clock) State() int {
	switch {
	case c.intro > c.t.CtrlTime+1:
		return 0
	case c.intro > 0:
		return 1
	case c.intro == 0:
		return 2
	case c.intro < -c.t.OverWaitTime:
		return 4
	}
	return 3
}

// Event is what a clock step asks the caller to do.
type Event int

const (
	// EventNone: nothing.
	EventNone Event = iota
	// EventFightStart: give the fighters control (and state 0).
	EventFightStart
	// EventPoses: send the fighters to their win, lose or draw poses.
	EventPoses
	// EventRoundOver: the round is done; start the next one.
	EventRoundOver
)

// Step advances the clock one tick (stepRoundState, system.go:3332-3470):
// the intro counts down, held at ctrl.time+1 while a fighter asserts intro;
// at 0 the fight starts and the clock waits there until the round is
// decided; then it counts down through RoundState 3 and 4, asking for the
// poses over.wintime ticks into RoundState 4 and ending the round
// over.time ticks after that.
// shortcut: the KO slow motion and the waits for fighters still moving are
// not modelled.
//
// settled reports every fighter at rest (in control, idle and standing, or
// knocked out): RoundState 4 waits for it (system.go:3441-3466).
func (c *Clock) Step(intro, decided, settled bool) Event {
	switch {
	case c.intro > c.t.CtrlTime:
		c.intro--
		if intro && c.intro <= c.t.CtrlTime {
			c.intro = c.t.CtrlTime + 1
		}
		return EventNone
	case c.intro > 0:
		c.intro--
		if c.intro == 0 {
			return EventFightStart
		}
		return EventNone
	case c.intro == 0 && !decided:
		return EventNone
	}
	c.intro--
	if c.intro == -c.t.OverWaitTime-1 && !settled {
		c.intro = -c.t.OverWaitTime
	}
	if c.State() == 4 && !c.posed {
		c.winPose--
		if c.winPose <= 0 {
			c.posed = true
			return EventPoses
		}
	}
	if c.posed && c.intro < -(c.t.OverWaitTime+c.t.OverWinTime+c.t.OverTime) {
		return EventRoundOver
	}
	return EventNone
}
