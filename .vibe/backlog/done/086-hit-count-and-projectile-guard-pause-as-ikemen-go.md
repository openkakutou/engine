---
status: done
---
# Hit count after p1stateno, and a projectile's guard pause, as Ikemen GO

## Description
Found while fixing 077 (2026-11-23).
- The striker's hit count is added at the hit (`applyHitReaction`, `tick.go`), before `p1stateno`'s state entry resets it (`hitcountpersist`). Ikemen GO adds it at the end of the tick, after the state change (char.go:13125-13129), so after a `p1stateno` into a state without `hitcountpersist` `HitCount` is 1 there and 0 here.
- A guarded projectile pauses for `pausetime` (`projectile.go`, `p.HitPause = max(params.PauseTime[0], 0)`); Ikemen GO uses `guard.pausetime` on a guard, and Winmugen characters' projectile pauses are one tick shorter (char.go:14527-14533).

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`).

## Acceptance Criteria
- [x] After a hit whose `p1stateno` enters a state without `hitcountpersist`, `HitCount` is 1, as in Ikemen GO
- [x] A guarded projectile pauses for `guard.pausetime`, minus one tick for a Winmugen character, as Ikemen GO does
- [x] Tests cover both

## Done
2026-11-24, v2.77.9. The cause of the hit count difference was the moment `p1stateno` enters its state: Ikemen GO's stateChange1 only changes the number (and the hit takes control), the header runs in stateChange2 once the hit pause is over (char.go:6943-6944). `pendStateChange` now does that through the engine's existing `StateEntryPending`, so the hit count stays 1 through the pause and the state's header (anim, hitcountpersist) applies after it. `projectileHitPause` uses `guard.pausetime` on a guard and one tick less for a WinMUGEN character. Tests: `TestTick_P1StateNoIsEnteredOnceTheHitPauseIsOver`, `TestTick_AProjectilesHitPauseIsShorterForAWinMugenCharacter`, `TestProjectileHitPause_GuardedUsesGuardPauseTime`, red before the fix.
