---
status: done
---
# One power cap, the character's own

## Description
Found by the audit of 2026-11-21 (high). The power cap is computed in several places that disagree: `maxPower` (`pause.go:77-83`) and `maxPowerOf` (`statemachine/misc.go:64-70`) read `[Data] power`, while the PowerAdd controller (`statemachine/statemachine.go:776-783`) and a Statedef's `poweradd` (`tick.go:1647`) clamp to the fixed `DefaultMaxPower` (3000); `evaluator/triggers.go:96` and `constants.go:160` repeat the 3000. For a character whose `power` is 1000 the meter overfills past `PowerMax`; with 5000, PowerAdd stops at 3000. Ikemen GO clamps every power change to the character's `powerMax`.

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`).

## Acceptance Criteria
- [x] One helper gives the cap; PowerAdd, PowerSet, the Statedef `poweradd`, SuperPause and the `PowerMax` trigger all use it
- [x] Tests with `power = 1000` and `power = 5000` pin PowerAdd and the Statedef `poweradd` at the character's cap
- [x] The `DefaultMaxPower` comment says it is the fallback; a decision supersedes the wording of decision 015 ("no second data source")

## Done
2026-11-23 - v2.77.3, decision 035: `evaluator.Context.PowerMax()` is the one cap (`TestPowerAdd_IsCappedAtTheCharactersOwnPower`, `TestPowerMaxTrigger_IsTheSameCap`, `TestTick_StatedefPowerAdd_IsCappedAtTheCharactersPower`).
