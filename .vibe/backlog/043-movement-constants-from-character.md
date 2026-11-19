---
status: todo
---
# Movement constants and life from the character

## Description
The engine uses its own `TickConfig.Gravity` as every fighter's `yaccel`, has no stand or crouch friction, and caps health at a hardcoded `DefaultMaxHealth = 1000`. Ikemen GO reads these per character from the `.cns` `[Data]` (`life`) and `[Movement]` (`yaccel`, `stand.friction`, `crouch.friction`, thresholds) sections. Thread them from `character` into each fighter and apply them as Ikemen GO does, including friction for physics S and C (`src/char.go` posUpdate), which item 027 left out because no constants reached the engine.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO, cited from source, real characters.

## Acceptance Criteria
- [ ] Each fighter uses its own character's life, yaccel and friction values, with Ikemen GO's defaults when absent
- [ ] Physics S and C apply friction as Ikemen GO does, cited
- [ ] LifeAdd/LifeSet clamp to the character's life, not a constant

## Notes
Raised by item 027 (2026-11-18, ADR 020).
Item 044 (2026-11-19): gravity still comes from `TickConfig.Gravity`, not each character's `yaccel` (Constants). Do it here.
