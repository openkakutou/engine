---
status: done
---
# A helper's hit acts on the helper

## Description
Found by the audit of 2026-11-21 (medium). `hitWithHelpers` (`helper.go:183-192`) and `hitWithProjectiles` (`projectile.go:210-219`) call `applyHitReaction` with the attacker's side, so the attacker-side effects land on the player's runtime; they save and restore four fields, not `HitCount++` (`tick.go:1796`), the `p1facing` turn (1828), the `p1stateno` change (1831) nor the knockback facing (1814, 1847). A helper's HitDef with `p1stateno` sends the player into that state. In Ikemen GO the hitting char is the helper (char.go:12194-12209).

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`).

## Acceptance Criteria
- [x] `applyHitReaction` takes the striking context (player, helper or projectile owner as Ikemen GO defines it); the save-and-restore in both callers is gone
- [x] A helper's `p1stateno`, `p1facing` and hit count apply to the helper; the knockback direction uses the helper's facing
- [x] Tests cover a helper HitDef with `p1stateno` and one with `p1facing`

## Done
2026-11-23, v2.77.4. `applyHitReaction` takes a `striker` (side, runtime, projectile): the player or helper whose HitDef hit, or the projectile's owner. The attacker-side effects land on it; for a projectile, the contact, pause, hit count and `p1facing` are skipped (the projectile's own, char.go:14525-14535) and the knockback follows the projectile's facing (byf, char.go:11644-11651). The save-and-restore in `hitWithHelpers` and `hitWithProjectiles` and `ownerContact` are gone. Tests: `TestTick_AHelpersP1StateNoIsTheHelpers`, `TestTick_AHelpersHitCountsForTheHelper`, `TestTick_AHelpersP1FacingTurnsTheHelper`, `TestTick_AProjectilesHitLeavesItsOwnersFacingAndHitCount`, red before the fix.
Left as is: the hit count is added at the hit, before `p1stateno`'s state entry resets it; Ikemen GO adds it at the end of the tick (char.go:13125-13129), so a `p1stateno` into a state without `hitcountpersist` keeps 1 there, 0 here (player and helper alike). A projectile's guard pause is still `pausetime`, not `guard.pausetime` (char.go:14531).
