---
status: todo
---
# Tick Never Decrements RoundTimer

## Description
`MatchState.RoundTimer` (`match/state.go`) is set once at round start (`NewMatchState`) but nothing in the simulation loop ever counts it down: `Tick` advances animation/physics/state/round timers for both fighters but never touches `RoundTimer` itself. A round can currently only end by KO or double-KO — a real match timing out (time-limit expiry, the standard MUGEN/Ikemen "Time Over" outcome) can never happen, no matter how many ticks run.

Surfaced by `mode-quick-versus` while runtime-verifying its own backlog item 007 (round/match result screen): driving 900+ real ticks against the real `engine.wasm` build never produced a timeout, confirmed by reading this repo's own source rather than assumed from behavior alone.

## Acceptance Criteria
- [ ] `Tick` decrements `RoundTimer` by 1 each tick (never below 0), for both fighters' shared round state
- [ ] `RoundTimer` reaching 0 ends the round the same way a KO does today (a round-decided outcome consumers like `mode-quick-versus` already handle), with a distinct "time over" reason so a consumer can tell it apart from a KO/double-KO if it chooses to
- [ ] A `bestOf`/round-count match with `RoundTimer` reaching 0 while both fighters are still standing resolves to whichever life-total/tie rule MUGEN/Ikemen's own "Time Over" convention uses (highest remaining health wins; equal health is a draw) — not left undefined
- [ ] Existing `match`/`statemachine`/`tick` test suites pass unchanged; a `RoundTimer` of 0 configured at round start (an explicitly untimed round, matching MUGEN/Ikemen's own "unlimited time" convention) never ends a round on this path

## Notes
Cross-repo: unblocks nothing by itself, but `mode-quick-versus`'s own result screen (item 007, already shipped) already handles whatever round/match outcome `engine` reports — no consumer-side change expected once this ships, verified via the mutation-tested unit suite there rather than a live timeout (which this gap currently makes impossible to observe for real). See `mode-quick-versus`'s `docs/testing.md` for the original finding.
