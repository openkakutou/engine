---
status: done
---
# AnimTime after a state change matches Ikemen GO

## Description
After a ChangeAnim, AnimTime ends the tick at 1, because Ikemen GO advances the animation after the controllers (`src/char.go`, `anim.Action()`). Entering a state through ChangeState ends the tick at 0, with no advance. One of the two differs from Ikemen GO: check the source and make both follow it.

🔒 MUST: Ikemen GO behaviour, cited.

## Acceptance Criteria
- [x] AnimTime and AnimElem after a ChangeState and after a ChangeAnim match Ikemen GO on the same tick, cited and tested

## Notes
Raised by item 027 (2026-11-18).

## Progress
- 2026-11-21: already as Ikemen GO: the engine steps the animation at the end of every tick, a state change included (tick.go, after the run; Ikemen GO tick(), char.go:13055-13061). Covered by `TestTick_ANewAnimationHasPlayedOneTickAtTheEndOfItsTick` (ChangeAnim and a ChangeState header both end the tick at AnimElapsed 1).
