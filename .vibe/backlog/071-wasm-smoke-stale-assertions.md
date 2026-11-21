---
status: todo
depends_on: []
---
# WASM smoke harness: stale assertions

## Description
`node cmd/wasm/smoke.mjs` fails on main (checked 2026-11-21, v2.36.0): seven assertions describe behaviour the engine has since aligned with Ikemen GO (animTime on a state transition, a hit's damage and KO, the hit tick's PlaySnd and Explod, which is no longer unsupported). The harness is not run by `go test`, so nothing caught it.

## Acceptance Criteria
- [ ] Each failing assertion is updated to the current, Ikemen GO-grounded behaviour (or its fixture changed), never deleted
- [ ] The smoke runs in CI after the WASM build, so it cannot go stale again
