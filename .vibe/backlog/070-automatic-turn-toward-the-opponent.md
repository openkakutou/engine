---
status: todo
---
# Automatic Turn Toward the Opponent

## Description
Ikemen GO turns a fighter toward its opponent on its own when it is standing, crouching or walking with control and the opponent is behind it (autoTurn, char.go, entering the turn state 5 or 6 with the turn animation), unless the states assert `noautoturn`. The engine never turns a fighter by itself: after a cross-up both fighters can face the same way. `AssertSpecial noautoturn` is recorded (item 050) but has nothing to disable yet.

🔒 MUST: Ikemen GO behaviour, grounded in its source (commit `54d4148a`).

## Acceptance Criteria
- [x] A fighter whose opponent passes behind it turns as Ikemen GO does, with the source cited
- [x] `noautoturn` keeps it from turning
- [ ] End-to-end scenario on the reference kfm: a jump over the opponent ends with both fighters facing each other

## Notes
Found by item 050 (2026-11-21).
- 2026-11-21: autoTurn done (tick_autoturn_test.go on the reference kfm); left: the jump-over scenario, the opponent's noturntarget, the stage autoturn switch and a MUGEN character's turn on a ChangeState with control.
