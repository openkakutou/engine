---
status: todo
depends_on: [022]
---
# Guarding: block detection, guard damage and chip

## Description
Add blocking: a defender holding back (or guard command) when hit by a guardable HitDef takes the guard variant of the reaction, guard damage (chip) instead of full damage, and the guard pause/stun times. Unguardable and guard-flag-restricted hits (high/low) must be respected.

## Acceptance Criteria
- [ ] A defender holding back against a guardable hit enters a guard state and takes guard damage only
- [ ] High/low guard flags are honoured (a crouching guard does not block an overhead)
- [ ] Chip damage cannot reduce health below 1 unless the HitDef allows it
- [ ] Real-character fixture test covering stand guard, crouch guard and an unguardable hit
