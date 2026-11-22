---
date: 2026-11-23
status: accepted
---
# The power cap is the character's own [Data] power

**Context:** Backlog item 075 (audit of 2026-11-21). Decision 015 kept a hardcoded 3000 cap "until a real second data source exists"; the character's `[Data] power` has since been read by PowerSet and SuperPause, while PowerAdd and a Statedef's `poweradd` still clamped to 3000.

**Decision:** Every power change clamps to `evaluator.Context.PowerMax`: the character's `[Data] power`, else 3000 (`DefaultPowerMax`). PowerAdd, PowerSet, a Statedef's `poweradd`, SuperPause and the `PowerMax` trigger all read it. `statemachine.DefaultMaxPower` stays as an alias of the fallback.

**Reason:** Ikemen GO clamps every power change to the character's `powerMax`, read from `[Data] power` (setPower, char.go:9019-9029; char.go:4148).

**Rejected alternatives:** Keeping 3000 for PowerAdd only — a 1000-power character overfilled past its `PowerMax` and a 5000-power one stopped at 3000.

Supersedes the "no second data source" wording of decision 015; its reset of power to 0 at match start stands. Divergence kept: an explicit `power = 0` reads as 3000 here, 0 in Ikemen GO.
