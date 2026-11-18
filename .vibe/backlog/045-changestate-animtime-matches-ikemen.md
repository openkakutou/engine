---
status: todo
---
# AnimTime after a state change matches Ikemen GO

## Description
After a ChangeAnim, AnimTime ends the tick at 1, because Ikemen GO advances the animation after the controllers (`src/char.go`, `anim.Action()`). Entering a state through ChangeState ends the tick at 0, with no advance. One of the two differs from Ikemen GO: check the source and make both follow it.

🔒 MUST: Ikemen GO behaviour, cited.

## Acceptance Criteria
- [ ] AnimTime and AnimElem after a ChangeState and after a ChangeAnim match Ikemen GO on the same tick, cited and tested

## Notes
Raised by item 027 (2026-11-18).
