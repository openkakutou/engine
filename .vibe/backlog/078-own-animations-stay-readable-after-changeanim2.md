---
status: todo
---
# A fighter's own animations stay readable after ChangeAnim2

## Description
Found by the audit of 2026-11-21 (medium). `tickFighter` swaps `ctx.Anims` to the opponent's table when `AnimFromOpponent` is set (`tick.go:1039-1041`), once at the start of the tick. `SelfAnimExist` reads that table (`evaluator/animation.go:191`) where Ikemen GO always reads the character's own (char.go:5597-5602), and after a ChangeAnim2 or ChangeAnim in the middle of a state run, `AnimTime` / `AnimElem` read the wrong table for the rest of the tick. The comment at `animation.go:172-174` still says there is no ChangeAnim2.

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`).

## Acceptance Criteria
- [ ] The context keeps the fighter's own table apart from the shown one; `SelfAnimExist` reads the own table, `AnimExist` the shown one
- [ ] Animation triggers pick the table from `AnimFromOpponent` when they run, so a ChangeAnim2 followed by `AnimTime` in the same state reads the opponent's animation
- [ ] Tests cover both, on two characters whose animation sets differ
