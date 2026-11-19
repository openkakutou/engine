---
date: 2026-11-19
status: accepted
---
# Player push runs after both fighters move, before hit detection (item 063)

**Context:** Fighters walked through each other. Ikemen GO (commit `54d4148a`) pushes them apart in `CharList.collisionDetection`, before hit detection (`char.go:14954-14960`, issue 1941), which runs after every fighter's action (`system.go:2734`). `pushDetection` (`char.go:14551-14779`) needs both fighters' `playerpush` flag, their size boxes overlapping (by more than 0 on X) and their Clsn2 boxes overlapping (`char.go:14641-14645`, MUGEN behaviour). The overlap is shared by `weight` and the average `pushfactor`, or taken by the lower `PlayerPush` priority alone; positions are clamped to the stage, and a cornered fighter makes the other take the whole overlap times its own pushfactor. `actionPrepare` sets `playerpush`, priority 0 and affectteam 1 before every unfrozen tick's states (`char.go:12350-12361`), so the `PlayerPush` controller (`bytecode.go:11309-11327`) lasts one tick.

**Decision:**

```
 Tick(n)
   P1: states -> move -> land
   P2: states -> move -> land
      v
   pushFighters(P1 = c, P2 = getter)   size boxes, Clsn2, priority/weight, corner
      v
   hit detection on the pushed positions
```

- `push.go`'s `pushFighters` mirrors `pushDetection` for the one pair of a one-on-one match: P1 is `c` (lower player id), P2 the `getter`; the tie-break on equal X is Ikemen GO's (priority, move type H then A, height, facing).
- The size box is read from `Constants`: `size.air.*` for state type A, `size.ground.*` otherwise, `size.height` tall (`char.go:4444-4459`, `10484-10522`). Clsn2 overlap uses the frames hit detection uses, touching counts (`RectIntersectAABB`, `common.go:325-357`).
- `Context.PlayerPush`, `PushPriority`, `PushAffectTeam` carry the flags; `tickFighter` resets them when not frozen.

**Differences from Ikemen GO (accepted):**

| Subject | Ikemen GO | Engine |
|---|---|---|
| Crouch/down boxes | `crouch.sizebox`, `down.sizebox` (default: the ground box) | always the ground box; `air.sizebox` top ignored |
| Width/Height controllers, `sizepushonly`, Z axis, helpers | supported | none yet (no such features in the engine) |
| Screen bound | camera edges and stage bounds (`xScreenBound`) | stage bounds only, as `physics.Step` |
| Round state | playerpush only in round states 1-3 | `Tick` is always the fight |
| Precision | float32 | float64 |

**Rejected alternatives:** *Pushing in `physics.Step`:* it moves one fighter, and push needs both after both moved. *Pushing before P2 acts:* Ikemen GO pushes once, after all actions.
