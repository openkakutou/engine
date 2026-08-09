---
date: 2026-08-09
status: accepted
---
# The trigger evaluator reads a dedicated Context, not an extended FighterState

**Context:** Implementing backlog item 002 (trigger/expression evaluator for MUGEN CNS syntax). The built-in triggers this item must support at minimum (`Time`, `Anim`, `AnimTime`, `Ctrl`, `Command`, `var()`, `sysvar()`) need per-fighter runtime data — state timer, current animation/animation timer, control flag, active input commands, general-purpose variables — that `match.FighterState` (item 001) does not model yet. That data rightfully belongs to state-machine execution (item 003) and input/command matching (item 008), neither of which exists yet.

**Decision:** Introduce `evaluator.Context`, a struct owned by the `evaluator` package that embeds `match.FighterState` (for the fields item 001 already defined, e.g. `StateNo`) and adds the extra fields the supported built-ins need (`Time`, `Ctrl`, `Anim`, `AnimTime`, `Vars`, `SysVars`, `ActiveCommands`). Callers build a `Context` per fighter, per tick. `match.FighterState` itself is left untouched.

**Reason:** Keeps item 001's already-closed, already-tested API stable rather than reopening it for fields two other not-yet-built items actually own. Some of the new data (`ActiveCommands`) is a map, which is not comparable — adding it directly to `FighterState` would break that package's existing `==`/`!=` comparisons in its tests. A separate `Context` type also draws a clean seam: when items 003/008 land, they decide whether this data moves into `FighterState` permanently or stays evaluator-owned; nothing about this item's public API has to change either way.

**Rejected alternatives:**
- *Add the fields directly to `match.FighterState`* — rejected: reopens an already-closed item's data model for fields it doesn't own yet, and a map field would break `FighterState`'s existing struct-equality comparisons used throughout its tests.
- *Pass each piece of runtime data as a separate `Eval` parameter* — rejected: an unbounded, growing parameter list as more built-ins are added later (items 003/008); a single `Context` struct scales better.
