---
status: todo
---
# Fall and SuperPause defence multipliers

## Description
Found while fixing 080 (2026-11-23). Ikemen GO's final defence is `defence * customDefense * superDefenseMul * fallDefenseMul / 100` (char.go:12980-13001): a falling fighter takes less damage after each juggle hit (`[Data] fall.defence_up`), and a SuperPause's `p2defmul` raises the opponent's defence for the pause's hits. `evaluator.Context.FinalDefence()` has only `[Data] defence` and DefenceMulSet (`statemachine/pause.go:24` lists `p2defmul` as not modelled), so the hits and LifeAdd that divide by it are too strong in both cases.

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`).

## Acceptance Criteria
- [ ] A falling fighter's defence grows by `fall.defence_up` as Ikemen GO's fallDefenseMul does, and resets when it lands as Ikemen GO resets it
- [ ] A SuperPause's `p2defmul` scales the opponent's defence while Ikemen GO applies it (only once already hit, issue 3814)
- [ ] `FinalDefence()` includes both; tests cover a juggle and a super
