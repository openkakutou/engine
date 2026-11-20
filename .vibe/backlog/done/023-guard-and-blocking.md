---
status: done
depends_on: [022, 035]
---
# Guarding: block detection, guard damage and chip

## Description
Add blocking: a defender holding back (or guard command) when hit by a guardable HitDef takes the guard variant of the reaction, guard damage (chip) instead of full damage, and the guard pause/stun times. Unguardable and guard-flag-restricted hits (high/low) must be respected.

## Acceptance Criteria
- [x] A defender holding back against a guardable hit enters a guard state and takes guard damage only
- [x] High/low guard flags are honoured (a crouching guard does not block an overhead)
- [x] Chip damage cannot reduce health below 1 unless the HitDef allows it
- [x] Real-character fixture test covering stand guard, crouch guard and an unguardable hit

## Notes
Combat-completeness milestone, step order in roadmap decision `036`.
- 2026-11-21: done. Guard as Ikemen GO: SCF_guard after the states (back held or autoguard, by stance), the engine's guard entry into 120 within guard distance, guardflag H/L/A against the stance, guard damage scaled like a hit and capped at life - 1 with guard.kill 0, guard pause/hit/slide/ctrl times chained by character version, guard velocity, guard hit states 150/152/154, MoveGuarded. e2e_guard_test.go: standing guard vs the light kick (both common1), crouching guard vs the low sweep, standing guard failing against it. Not done: guard points / guard break, airguard landing details beyond 154, ReversalDef (MoveReversed).
