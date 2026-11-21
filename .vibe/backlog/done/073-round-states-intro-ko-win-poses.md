---
status: done
---
# Round states: intro, fight, KO, win poses

## Description
The engine only models the fight itself (RoundState 2): `RoundState` is a constant 2, the round ends at the KO tick and the fighters never play their intros or win poses. Model Ikemen GO's round flow (system.go:1908-1923, 2528, 3318-3470; fs.go:3298-3312, commit `54d4148a`): a round clock (`intro` counter) giving RoundState 0 (pre-intro), 1 (intros, held while a fighter asserts `intro`), 2 (fight: control given, fighters back to state 0 unless `nointroreset`), 3 (decided, the fighters still move) and 4, with the win, lose and draw poses (states 180, 170, 175) after over.wintime; the fighters enter state 5900 at round start; the win triggers read the decided round.

MUST: Ikemen GO behaviour (roadmap decision `038`).

## Acceptance Criteria
- [x] A round clock with Ikemen GO's timings (fight.def [Round] values, its defaults otherwise) gives RoundState 0 to 4 and the moments to start the fight, send the fighters to their poses and end the round
- [x] The RoundState trigger and the Win / Lose / DrawGame family read the round's state and result; the round timer counts and the outcome is decided only in RoundState 2
- [x] A fighter starts the round in state 5900 without control, gets control (and state 0) when the fight starts, and enters 180 / 170 / 175 for the poses
- [x] E2E: kfm plays its intro (state 191 on round 1), fights, and its win pose after a KO

## Notes
The KO slow motion (slow.time, slow.speed) and the lifebar's round announcements are left to the game (game backlog).

## Done
2026-11-21 - engine v2.45.0 (round.Clock, TickConfig.Round, NewRoundRuntime, StartFight, EnterPoses, Settled, IntroAsserted); e2e TestE2E_KfmRoundIntroFightKOAndWinPose. Left to the game: driving the clock, the KO slow motion, the lifebar announcements.
