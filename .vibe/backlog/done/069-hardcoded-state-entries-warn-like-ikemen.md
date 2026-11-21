---
status: done
depends_on: [068]
---
# Hard-coded state entries and negative states as Ikemen GO

## Description
Follow-up of 068. Three engine-driven state entries still skip silently when the target state is missing, where Ikemen GO enters an empty state and warns (`src/char.go:6880-6887` @54d4148a): the basic walk/crouch/jump changes (`basicChangeState`, states 10, 12, 20, 40, 0), the landing into 52 (`land`), and the hit reaction into 5000/5010/5020 (`applyHitReaction`). Route them through 068's empty-state entry and warning. Also: Ikemen warns "attempted to change to negative state" and clamps the target to 0 (`char.go:6851-6857`, `Max(0, no)`); this engine does neither.

🔒 MUST: as Ikemen GO (roadmap decision `041`).

## Acceptance Criteria
- [x] Each of the three entries to a missing state enters an empty state with a warning, covered end to end through `Tick`
- [x] A change to a negative state warns and goes to state 0, as Ikemen GO

## Progress
- 2026-11-21 (v2.76.0): a ChangeState to a negative state warns and goes to 0 (stateChange1, char.go:6812, 6855-6860). Left: the three engine-driven entries into a missing state.
- 2026-11-21 (v2.77.0): the basic changes (10, 12, 20, 40, 0, 120), the landing into 52 and the get-hit change into 5000/5010/5020 enter a missing state empty with Ikemen GO's warning (`warnIfMissing`); through Tick: `TestTick_LandedHit_WithoutTheGetHitState_FreezesTheDefenderAndDoesNotFail`, the conformance run and the standing-only scenarios.
