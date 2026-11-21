---
status: todo
depends_on: [029]
---
# Explod lifecycle

## Description
Explods are reported today as one-tick `Explod` events. Keep them as live entities like projectiles and helpers: animation, position (postype, bind), velocity and acceleration, removetime, removeongethit, ModifyExplod and RemoveExplod, NumExplod, so the game can draw them for their whole life. Split from item 029.

MUST: Ikemen GO behaviour (roadmap decision `038`), commit `54d4148a`.

## Acceptance Criteria
- [x] Explod instances are reported per tick (animation, position, remaining time) so the game can draw them
- [ ] RemoveExplod (done), ModifyExplod, removetime and removeongethit behave as in Ikemen GO; NumExplod counts them
- [x] A cap on live explods prevents unbounded growth (512, Ikemen GO ExplodMax; Ikemen GO makes none past it, without a warning)
- [x] End-to-end scenario on a real character's explod

## Progress
2026-11-21 - engine v2.42.0: Explod, RemoveExplod, NumExplod(id), postypes, bind, velocity/accel, removetime (-2 until the animation ends), removeongethit, 512 cap, WASM `explods`; e2e `TestE2E_KfmsIntroWoodPiecesAreExplods`. Left: ModifyExplod, ExplodBindTime, space, scale, trans, friction.
