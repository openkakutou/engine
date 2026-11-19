---
date: 2026-11-19
status: accepted
---
# A ChangeState by the current state runs the new state in the same tick (item 064)

Supersedes the "State 52's first run" row of 029 and its rejected alternative.

**Context:** Ikemen GO (commit `54d4148a`) enters a new state and runs it at once: `changeStateEx` loops while the run changes state again, up to `MaxLoop` = 2500 (`char.go:6966-6994`, `bytecode.go:15`), the ChangeState of the last run being refused (`stateChange1`, `char.go:6803-6807`). Time grows once, at the end of the tick (`char.go:12592`). The engine ran a newly entered state on the next tick, so every state change was one tick late; item 044 worked around it for the landing.

**Decision:** `stateRuns.followChanges` (`tick.go`) enters the state a ChangeState named and runs it, again for every ChangeState that run makes, up to `maxStateLoop` runs; the last run's ChangeState is refused by restoring `StateNo`, `PrevStateNo`, `Time` and the persistent counters. In a hit pause the entry waits (`StateEntryPending`, `char.go:6943`) but the new state's `ignorehitpause` controllers still run. A state entered this tick shows its first animation frame to collision detection and its HitDef is reported this tick. `Time` and `AnimElapsed` grow once, after the move and the landing. `land` no longer grounds the fighter itself: it changes to state 52, which `followChanges` runs, so state 52's own `VelSet`/`PosSet` at Time 0 do it (`common1.cns:273-281`).

**Expected values that changed (Ikemen GO reason):** a state entered by ChangeState reads `Time` 1 and `AnimElapsed` 1 at the end of its change tick (it ran at Time 0, then grew once, `char.go:12592`) instead of 0 and 0; the get-hit state 5001 runs its controllers first on the tick of the 5000 to 5001 change; a landing at Time 0 ends with `Time` 1; a hand-built state 52 must carry the ground controllers to ground the fighter.

**Rejected alternatives:** *Keeping one exception for the landing:* two state-entry timings in one engine.
