---
status: todo
---
# Projectile spawn, movement and collision

## Description
Support the Projectile controller: a state spawns a projectile with its own animation, velocity and hit definition; each tick it moves, can hit the opponent through the existing hit-detection pipeline, and is removed on hit, on leaving the stage or when its lifetime ends. Expose projectile positions and animation numbers through the WASM response so a game can draw them.

## Acceptance Criteria
- [ ] A Projectile controller spawns a projectile owned by the casting fighter
- [ ] Projectiles move each tick and hit the opponent through hitdetect, applying damage like a melee hit
- [ ] Projectiles are removed on hit, off-stage or on expiry, with no leak across rounds
- [ ] WASM match update reports each live projectile (position, facing, animation number)
- [ ] Fixture test with a real character's fireball

## Notes
Cross-repo: mode-quick-versus must render the new projectile data (tracked in that repo once this lands).
