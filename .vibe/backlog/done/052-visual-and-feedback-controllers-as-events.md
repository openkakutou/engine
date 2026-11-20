---
status: done
depends_on: [054]
---
# Visual and Feedback Controllers as Events

## Description
Accept PalFX, RemapPal, MakeDust, GameMakeAnim, ForceFeedback and a minimal Explod, and report each one as an event in the tick result, so that common1 runs. The engine only reports these events; the game draws them. The full helper and explod lifecycle stays in item 029.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`), grounded in its source (commit `54d4148a`).

## Acceptance Criteria
- [x] Each type is accepted and produces an event with its evaluated parameters
- [x] The WASM tick response carries these events
- [x] The conformance run no longer reports these types as unsupported

## Notes
Split out of item 035 (Product Owner, 2026-11-18): the reference files are the `common1.cns` and `common1.cns.zss` shipped with Ikemen GO (MIT, roadmap decision `037`); acceptance characters are Ikemen GO's `kfm` and `kfm_zss`.

- 2026-11-21: also FallEnvShake (from item 049): the HitDef's fall.envshake.* reported as a screen-shake event when the controller runs.
- 2026-11-21: done. PalFX, RemapPal, MakeDust, GameMakeAnim, ForceFeedback, Explod, EnvShake and FallEnvShake are ControllerEvents (TickResult.Events, the WASM tick events); SprPriority and Width (edge) are applied. The conformance run on kfm reports no unsupported controller.
