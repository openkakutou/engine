---
status: todo
---
# Expose Current Animation Number And Frame Timing Via WASM

## Description
The WASM `tick`/`newMatch`/`resetRound` response's `state` (`match.MatchState`) exposes each fighter's `Position`, `Facing`, `Velocity`, `StateNo`, and `Health`, but never the resolved animation number or the elapsed time within that animation (`FighterRuntime.Context.Anim`/`AnimTime`, kept Go-side per `.vibe/decisions/011`). `StateNo` alone is not a reliable stand-in for which animation is playing: `cns.StateDef.Anim` (set via `changeanim`) can legitimately differ from the state number for real characters. A `mode-*` game app driving match rendering has no WASM-exposed way to know which animation and frame a fighter is actually displaying right now. Add the currently resolved animation number and enough frame-timing information to the JSON response so a consumer can pick the correct sprite without re-deriving state-machine logic itself.

## Acceptance Criteria
- [ ] A JS caller can read each fighter's current animation number and enough timing info (e.g. elapsed `AnimTime`) to resolve the exact frame from `tick`'s response, without a Go toolchain
- [ ] `newMatch`'s and `resetRound`'s responses expose the same for each fighter's starting state
- [ ] This new data is additive only to the response shape — no `FighterRuntime`/`input.State`/`combat.ComboState` field newly round-trips through JSON on the request side, and every existing `tick`/`newMatch`/`resetRound` field is unchanged
- [ ] Verified by the existing Node.js smoke harness (`cmd/wasm/smoke.mjs`) with a new assertion reading the added field(s)

## Notes
Cross-repo blocker for `mode-quick-versus` backlog item `005` (Match Rendering), which cannot pick the correct sprite/frame for either fighter without this — discovered while starting that item's implementation on 2026-08-31. `mode-quick-versus`'s item stays `status: blocked` until this lands and is published (tagged release), not merely `done` in this repo (see `roadmap`'s `/vibe:next-task` done-vs-published distinction).
