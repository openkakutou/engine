---
date: 2026-11-19
status: accepted
---
# Each fighter moves right after its states, then lands; hit detection after both (item 044)

**Context:** `Tick` ran both fighters' states, then hit detection, then `physics.Step` for both, which added gravity before moving. Ikemen GO (commit `54d4148a`) does otherwise: `CharList.action` runs each fighter's `actionRun` in turn (`char.go:14080-14084`); inside it, after the states, `posUpdate` (`char.go:12583`) moves the position by the velocity, then adds `yaccel` in physics A (`char.go:10102-10139`, `gravity` `char.go:9963-9965`); then a physics A fighter whose velocity points down, at or below the ground, outside state 105, changes to state 52 (`char.go:12584-12590`). Collision detection runs after all actions (`system.go:2734`). Consequences in the engine: a jump never landed (end-to-end jump scenarios failed), fighter 2 read fighter 1's position before its move, a `VelSet` moved the fighter after hit detection, and jump arcs peaked lower (76.0 instead of 84.4 for Kung Fu Man).

**Decision:**

```
 Tick(n)
   P1: states (-3, -2, -1, current) -> enter changed state -> move -> land
      |                                                (physics.Step) (state 52)
      v
   P2: same, seeing P1 after its move
      v
   hit detection, damage, applyHitReaction
      v
   round timer and outcome
```

- `physics.Step` is Ikemen GO's `posUpdate`: position += velocity, X clamped to the stage, then velocity Y -= gravity (`gravityFor`: the configured gravity in physics A, 0 otherwise). It no longer stops a fighter at the ground: like Ikemen GO, the ground is the landing's business.
- `tickFighter` calls it after the state change of the tick is entered (Ikemen GO runs a new state at once, so its physics type is the one `posUpdate` uses), then `land`: the Ikemen GO condition above changes to state 52 through `enterState`, like any ChangeState.
- A frozen fighter (hit pause) does not move, as `posUpdate` sits inside `!c.hitPause()` (`char.go:12573`).

**Differences from Ikemen GO (accepted):**

| Subject | Ikemen GO | Engine |
|---|---|---|
| State 52's first run | at once, in the landing tick (`changeStateEx`, `char.go:6987-6994`); common1's VelSet y = 0 and PosSet y = 0 at Time 0 put the fighter on the ground (`common1.cns:273-281`) | on the next tick, like every ChangeState in the engine; `land` sets Y and velocity Y to 0 itself so the landing tick's position is Ikemen GO's; horizontal velocity is kept, as there |
| Character without state 52 | enters an empty state | stays in its state, falling (`basicChangeState`'s shortcut) |
| Ground level | `groundLevel` and platforms (`char.go:12587`) | 0 |
| Gravity value | the character's `movement.yaccel` | `TickConfig.Gravity`; the reference scenarios pass kfm's .44, so the landing is exact. Reading `Constants` yaccel per character is left to a follow-up |
| Hit velocity | applied by the next tick's `posUpdate` | same: `applyHitReaction` runs after the move |

**Rejected alternatives:** *Keeping the ground clamp in `physics.Step`:* it zeroes the velocity before the landing check can see the fighter falling, and Ikemen GO has no such clamp. *Running state 52's controllers immediately:* the engine enters every other state for the next tick; one exception for the landing would make two state-entry timings.
