---
status: todo
---
# Get-Hit and Fall Controllers

## Description
Support HitVelSet, HitFallSet, HitFallVel, HitFallDamage, FallEnvShake, PosFreeze and SelfState, as common1's get-hit, fall, lie-down and get-up states use them.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`), grounded in its source (commit `54d4148a`).

## Acceptance Criteria
- [ ] Each controller changes the fighter's state as Ikemen GO does, with the source cited and a test using a value derived independently
- [ ] FallEnvShake is reported as an event, and the engine does not shake the screen itself
- [ ] The conformance run no longer reports these types as unsupported

## Notes
Split out of item 035 (Product Owner, 2026-11-18): the reference files are the `common1.cns` and `common1.cns.zss` shipped with Ikemen GO (MIT, roadmap decision `037`); acceptance characters are Ikemen GO's `kfm` and `kfm_zss`.
