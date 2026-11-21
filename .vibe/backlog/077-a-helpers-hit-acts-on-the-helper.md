---
status: todo
---
# A helper's hit acts on the helper

## Description
Found by the audit of 2026-11-21 (medium). `hitWithHelpers` (`helper.go:183-192`) and `hitWithProjectiles` (`projectile.go:210-219`) call `applyHitReaction` with the attacker's side, so the attacker-side effects land on the player's runtime; they save and restore four fields, not `HitCount++` (`tick.go:1796`), the `p1facing` turn (1828), the `p1stateno` change (1831) nor the knockback facing (1814, 1847). A helper's HitDef with `p1stateno` sends the player into that state. In Ikemen GO the hitting char is the helper (char.go:12194-12209).

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`).

## Acceptance Criteria
- [ ] `applyHitReaction` takes the striking context (player, helper or projectile owner as Ikemen GO defines it); the save-and-restore in both callers is gone
- [ ] A helper's `p1stateno`, `p1facing` and hit count apply to the helper; the knockback direction uses the helper's facing
- [ ] Tests cover a helper HitDef with `p1stateno` and one with `p1facing`
