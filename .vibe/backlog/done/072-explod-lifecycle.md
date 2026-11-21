---
status: done
depends_on: [029]
---
# Explod lifecycle

## Description
Explods are reported today as one-tick `Explod` events. Keep them as live entities like projectiles and helpers: animation, position (postype, bind), velocity and acceleration, removetime, removeongethit, ModifyExplod and RemoveExplod, NumExplod, so the game can draw them for their whole life. Split from item 029.

MUST: Ikemen GO behaviour (roadmap decision `038`), commit `54d4148a`.

## Acceptance Criteria
- [x] Explod instances are reported per tick (animation, position, remaining time) so the game can draw them
- [x] RemoveExplod (done), ModifyExplod, removetime and removeongethit behave as in Ikemen GO; NumExplod counts them
- [x] A cap on live explods prevents unbounded growth (512, Ikemen GO ExplodMax; Ikemen GO makes none past it, without a warning)
- [x] End-to-end scenario on a real character's explod

## Progress
2026-11-21 - engine v2.42.0: Explod, RemoveExplod, NumExplod(id), postypes, bind, velocity/accel, removetime (-2 until the animation ends), removeongethit, 512 cap, WASM `explods`; e2e `TestE2E_KfmsIntroWoodPiecesAreExplods`. Left: ModifyExplod, ExplodBindTime, space, scale, trans, friction.
2026-11-21 - engine v2.54.0 to v2.74.0: ModifyExplod (v2.54.0, with MUGEN's postype lock and Ikemen GO's relative times), scale (v2.55.0), trans / alpha (v2.66.0), ExplodBindTime (v2.74.0), pausing with Pause / SuperPause (v2.52.0). Left as shortcuts in code: space, random, friction, angle and the Ikemen GO extensions.
