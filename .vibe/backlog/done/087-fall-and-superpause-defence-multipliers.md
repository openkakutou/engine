---
status: done
---
# Fall and SuperPause defence multipliers

## Description
Found while fixing 080 (2026-11-23). Ikemen GO's final defence is `defence * customDefense * superDefenseMul * fallDefenseMul / 100` (char.go:12980-13001): a falling fighter takes less damage after each juggle hit (`[Data] fall.defence_up`), and a SuperPause's `p2defmul` raises the opponent's defence for the pause's hits. `evaluator.Context.FinalDefence()` has only `[Data] defence` and DefenceMulSet (`statemachine/pause.go:24` lists `p2defmul` as not modelled), so the hits and LifeAdd that divide by it are too strong in both cases.

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`).

## Acceptance Criteria
- [x] A falling fighter's defence grows by `fall.defence_up` as Ikemen GO's fallDefenseMul does, and resets when it lands as Ikemen GO resets it
- [x] A SuperPause's `p2defmul` scales the opponent's defence while Ikemen GO applies it (only once already hit, issue 3814)
- [x] `FinalDefence()` includes both; tests cover a juggle and a super

## Done
2026-11-24, v2.77.10. `Context.FallDefenceMul` grows on the first acted tick of 5070/5100 (not with `nofalldefenceup`), `SuperDefenceMul` takes a SuperPause's `p2defmul` when the opponent is in MoveType H at the end of the pausing tick; both reset out of a hit and enter `FinalDefence()`. Correction to the description: the fall multiplier grows per landing (5070/5100), not per juggle hit, as char.go:12949-12953 does.
