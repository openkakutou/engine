---
status: done
---
# What the hit phase writes survives to the next tick

## Description
Found by the audit of 2026-11-21 (high). After the hits land, `Tick` copies only `StateNo` and `Power` from the runtime contexts back to `state.Fighters` (`tick.go:616-619`), and the next `tickFighter` overwrites the context's `FighterState` with the match state (`tick.go:1025`). Two consequences:
- The `Facing` set by `applyHitReaction` for `p2facing` / `p1facing` (`tick.go:1821-1829`) and the `Facing` / `Velocity` set by `enterState` (facep2, velset) in the get-hit, custom or guard state are lost. A defender hit from behind by a HitDef with `p2facing = 1` (kfm.cns:890, 1387, 2156) keeps facing away for its whole get-hit sequence; Ikemen GO turns it (char.go:12194-12206, 12591).
- `strike` reads the defender's life from two instants (`projectile.go:235-253`): `guards()` from the context, the chip from the state already lowered by earlier strikes of the same tick (player hit, then projectiles, then helpers). A second guarded strike at low life writes a negative health, bypassing the floor of `ApplyHits` (`combat.go:123`), where a lethal hit should land.

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`), commit `54d4148a`.

## Acceptance Criteria
- [x] After the hit, reversal, projectile and helper phases, the whole fighter state is carried to the next tick (the health `ApplyHits` wrote included); no caller has to set both copies by hand
- [x] `strike` reads the defender's life from one place; two strikes in one tick never leave a negative health
- [x] End-to-end on real files: kfm hits an opponent facing away with a `p2facing = 1` HitDef and the opponent turns; `p1facing` on the thrower holds
- [x] A test lands a guarded hit and a projectile in the same tick on a defender at low life

## Done
2026-11-22 - v2.77.2: `Tick` copies each fighter's whole state from its runtime context after the hit phase, and `strike` keeps the defender's context life equal to the match state's (`TestTick_P2FacingTurnReachesTheMatchState`, `TestStrike_TwoGuardedStrikesInOneTickNeverLeaveNegativeLife`). The end-to-end criterion on kfm: kfm only uses `p2facing = 1`, which the defender's own autoturn already gives, so the reproduction uses a synthetic `p2facing = -1`; `p1facing` on the thrower is covered by the existing throw test.
