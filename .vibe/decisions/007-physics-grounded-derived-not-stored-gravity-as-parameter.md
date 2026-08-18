---
date: 2026-08-18
status: accepted
---
# Physics: ground/air state is derived, not stored; gravity and stage bounds are call-time parameters

**Context:** Implementing per-tick physics (backlog item 005) requires deciding how a fighter's ground/air state is tracked, since `match.FighterState` has no such field, and how the stage-boundary/gravity inputs `stage`'s CLAUDE.md and this item's own notes call out as read-only external data are threaded into the simulation.

**Decision:** `physics.Step` derives "grounded" each tick from the fighter's own `Position.Y`/`Velocity.Y` (`Y <= 0 && Velocity.Y <= 0`) rather than reading or writing a new stored field — matching decision 002's precedent of wrapping rather than reopening an already-closed data model. `Position.Y` follows an "upward is positive" convention: `0` is ground level, positive is height above it. Gravity is a required `float64` parameter (no built-in constant), and stage boundary data is a `*stage.StageBoundaries` parameter — `nil` means "not supplied" and produces the descriptive error the acceptance criteria call for, distinguishing that case from a legitimately zero-width boundary.

**Reason:** A derived ground/air check keeps `match.FighterState`'s already-closed API untouched and avoids a second source of truth that could drift out of sync with position/velocity. Treating `Velocity.Y > 0` at `Y == 0` as airborne (not grounded) makes gravity apply starting the same tick a jump's upward impulse is introduced, matching "leaving the ground" from the acceptance criteria. Gravity has no single canonical MUGEN-wide value — it is normally tuned per character/engine build — so hardcoding one would bake in an arbitrary game-balance number instead of leaving it to whichever later item wires physics into real match ticks. `*stage.StageBoundaries` mirrors the existing `nil`-is-a-well-defined-default convention already used for `evaluator.Context.ActiveCommands`.

**Rejected alternatives:**
- *Add a `Grounded bool` field to `match.FighterState`*: rejected — reopens an already-closed item's API for data fully derivable from fields it already has.
- *Hardcode a fixed gravity constant*: rejected — no established value exists anywhere in the org's docs; a wrong guess would be silently load-bearing for every future combat tuning decision.
- *Take `stage.StageBoundaries` by value with a zero value meaning "not supplied"*: rejected — a real stage can have `Left == Right == 0` as a legitimate (if degenerate) boundary, so a value type cannot distinguish "not supplied" from "supplied and empty" the way the acceptance criteria require.
