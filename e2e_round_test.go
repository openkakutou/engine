package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
	"github.com/openkakutou/engine/v2/round"
)

// A real kfm round as Ikemen GO plays it (system.go:2528-2595, 3318-3536):
// kfm enters 5900, then 190 and its intro 191 (kfm.cns:170-212, asserting
// Intro, which holds RoundState 1); the fight starts with control in state
// 0; after a KO the clock goes through RoundState 3 and 4 and the winner
// enters 180, kfm's win pose 181.
func TestE2E_KfmRoundIntroFightKOAndWinPose(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	var programs [2]FighterProgram
	var runtimes [2]FighterRuntime
	starting := [2]match.FighterState{
		{Side: match.SideP1, Position: match.Position{X: -70}, Facing: match.FacingRight, Health: 1000},
		{Side: match.SideP2, Position: match.Position{X: 70}, Facing: match.FacingLeft, Health: 1000},
	}
	for i := range runtimes {
		programs[i] = prog
		rt, err := prog.NewRoundRuntime(starting[i])
		if err != nil {
			t.Fatal(err)
		}
		runtimes[i] = rt
	}
	if runtimes[0].Context.StateNo != 5900 || runtimes[0].Context.Ctrl {
		t.Fatalf("round start: state %d ctrl %v, want 5900 without control", runtimes[0].Context.StateNo, runtimes[0].Context.Ctrl)
	}
	st, err := match.NewMatchState(1, 99*60, runtimes[0].Context.FighterState, runtimes[1].Context.FighterState)
	if err != nil {
		t.Fatal(err)
	}
	state := *st
	clock := round.NewClock(round.DefaultTimings)
	progress := RoundProgress{}
	var result round.RoundResult
	saw191, fightTick, poseTick, overTick := false, 0, 0, 0
	for tick := 1; tick <= 1500 && overTick == 0; tick++ {
		progress.State, progress.Result = clock.State(), result
		res, err := Tick(state, programs, runtimes, [2]input.TickInput{}, TickConfig{Bounds: testBounds(), Tick: tick, ComboWindow: 60, Round: &progress})
		if err != nil {
			t.Fatalf("tick %d: %v", tick, err)
		}
		state, runtimes = res.State, res.Fighters
		saw191 = saw191 || (runtimes[0].Context.StateNo == 191 && clock.State() == 1)
		if res.Round.Outcome != round.OutcomeNone {
			result = res.Round
		}
		// The KO: P2 falls at the 20th tick of the fight.
		if fightTick > 0 && tick == fightTick+20 {
			state.Fighters[match.SideP2].Health = 0
			runtimes[1].Context.Health = 0
		}
		intro := IntroAsserted(&runtimes[0]) || IntroAsserted(&runtimes[1])
		settled := Settled(&runtimes[0]) && Settled(&runtimes[1])
		switch clock.Step(intro, result.Outcome != round.OutcomeNone, settled) {
		case round.EventFightStart:
			fightTick = tick
			if err := StartFight(programs, &state, &runtimes); err != nil {
				t.Fatal(err)
			}
			if !runtimes[0].Context.Ctrl || runtimes[0].Context.StateNo != 0 {
				t.Errorf("fight start: state %d ctrl %v, want 0 with control", runtimes[0].Context.StateNo, runtimes[0].Context.Ctrl)
			}
		case round.EventPoses:
			poseTick = tick
			if err := EnterPoses(programs, &state, &runtimes, result); err != nil {
				t.Fatal(err)
			}
		case round.EventRoundOver:
			overTick = tick
		}
	}
	if !saw191 {
		t.Error("kfm never played its intro (state 191) in RoundState 1")
	}
	if fightTick == 0 || result.Outcome != round.OutcomeKO || result.Winner != match.SideP1 {
		t.Fatalf("fight tick %d, result %+v; want a fight won by P1 on a KO", fightTick, result)
	}
	if poseTick == 0 {
		t.Fatal("the poses never came")
	}
	if s := runtimes[0].Context.StateNo; s != 181 {
		t.Errorf("the winner's state after the poses = %d, want 181 (180 goes on to kfm's 181)", s)
	}
	if overTick == 0 {
		t.Error("the round never ended")
	}
}
