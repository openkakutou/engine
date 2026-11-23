---
status: done
---
# `guard.pausetime` with one value, and LifeAdd scaled by defence

## Description
Found by the audit of 2026-11-21 (medium). Two values the engine computes differently from Ikemen GO:
- `combat/combat.go:387-390`: a `guard.pausetime` with a single value sets the defender's guard shake to 0, overwriting the default copied from `pausetime`. Ikemen GO leaves the missing second value unset and fills it from `pausetime[1]` (bytecode.go:8479-8483, char.go:933-934).
- `statemachine/movement.go:309-337`: LifeAdd ignores `absolute` and does not scale by defence (the comment says "the engine has no defense"; it has, `damageMultiplier`). Ikemen GO's `lifeAdd` divides by the final defence unless `absolute`, with a minimum of -1 for a loss between 0 and -1 (char.go:8921-8931).

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`).

## Acceptance Criteria
- [x] A guarded HitDef with `pausetime = 12,12` and `guard.pausetime = 10` shakes the defender 12 ticks
- [x] LifeAdd on a fighter with a DefenceMulSet takes the scaled value; `absolute = 1` takes it raw

## Done
2026-11-23, v2.77.7. `ParseHitDef` sets `guard.pausetime`'s second value only when written. `Context.FinalDefence()` holds the defence `damageMultiplier` already used; LifeAdd divides by it unless `absolute` (or a MUGEN character's healing), with Ikemen GO's -1 minimum and rounding. Tests: `TestParseHitDef_GuardPauseTimeWithOneValueKeepsTheShake`, `TestApplyController_LifeAdd_IsScaledByDefenceUnlessAbsolute`, `…_ScaledLossIsAtLeastOne`, `…_MugenHealingIsAbsolute`, red before the fix.
Left as is: Ikemen GO's final defence also multiplies by the super pause and fall defence multipliers (char.go:13001), which neither hits nor LifeAdd model yet; the MUGEN healing rule reads the fighter's version, where Ikemen GO reads the state owner's.
