---
status: done
depends_on: []
---
# WASM smoke harness: stale assertions

## Description
`node cmd/wasm/smoke.mjs` fails on main (checked 2026-11-21, v2.36.0): seven assertions describe behaviour the engine has since aligned with Ikemen GO (animTime on a state transition, a hit's damage and KO, the hit tick's PlaySnd and Explod, which is no longer unsupported). The harness is not run by `go test`, so nothing caught it.

## Acceptance Criteria
- [x] Each failing assertion is updated to the current, Ikemen GO-grounded behaviour (or its fixture changed), never deleted
- [x] The smoke runs in CI after the WASM build, so it cannot go stale again

## Progress
- 2026-11-21 (v2.77.1): the fixture's HitDef gets an attr (Ikemen GO hits only with one) and its controllers run on Time = 1 (a new state runs in the tick it is entered); animTime reads 1 after the transition and 2 on the hit tick (the animation steps after every tick); the unsupported-controller check uses DisplayToClipboard, Explod being supported. The release workflow runs the smoke after building the WASM module; a CI on push is item 032.
