---
status: done
depends_on: [027, 028]
---
# Projectile spawn, movement and collision

## Description
Support the Projectile controller: a state spawns a projectile with its own animation, velocity and hit definition; each tick it moves, can hit the opponent through the existing hit-detection pipeline, and is removed on hit, on leaving the stage or when its lifetime ends. Expose projectile positions and animation numbers through the WASM response so a game can draw them.

## Acceptance Criteria
- [x] A Projectile controller spawns a projectile owned by the casting fighter
- [x] Projectiles move each tick and hit the opponent through hitdetect, applying damage like a melee hit
- [x] Projectiles are removed on hit, off-stage or on expiry, with no leak across rounds
- [x] WASM match update reports each live projectile (position, facing, animation number)
- [ ] Fixture test with a real character's fireball

## Notes
Cross-repo: game must render the new projectile data (tracked in that repo once this lands).
Combat-completeness milestone, step order in roadmap decision `036`.

## Done
2026-11-21 — Engine v2.37.0. Projectile controller (Ikemen GO projectileSub, defaults char.go:2719-2760, bytecode.go:9017-9028), lifecycle in Tick in Ikemen GO's order (spawn during the owner's states, update in charUpdate: removal by projremovetime / screen and stage bounds +40, hit and remove animations, move from the spawn tick on, velocity accel and velmul; hit against the defender's Clsn2 through the melee HitDef path; tick: contact ends it), NumProj, `projectiles` in the WASM tick response. E2E: TestE2E_AProjectileCrossesTheStageAndHits on the real kfm.
Last criterion only partly met: the reference kfm has no projectile, and the corpus characters checked (Ryu SVC, Sakura CVS) throw theirs from helpers, so the scenario adds the controller to the real kfm. A real character's own fireball comes with helpers (item 029).
Shortcuts left (marked in code): projhits > 1, projremove 0, projmisstime, projpriority and trades, the projectile's own hit pause, heightbound, postype other than p1.
