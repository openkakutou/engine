---
status: todo
depends_on: [007, 008]
---
# Round/Match Flow, WASM Entrypoint, Integration Tests

## Description
Tie every prior item together into a usable match loop, within `engine`'s combat-simulation boundary (win conditions, round reset, match-level bookkeeping — never menus, character selection, or overall game flow, which stay `mode-*` territory per `roadmap`'s `.vibe/decisions/004`/`008`). Implement round/match flow: win conditions (KO when a fighter's health reaches zero, timeout when the round timer expires with a health-based tiebreak), round reset (restoring both fighters to their starting state/position/health for the next round), and match-level progression (best-of-N rounds, tracking who won each round). Add a WASM build entrypoint (`GOOS=js GOARCH=wasm`, following `character`'s `cmd/wasm` precedent: build-tag-gated, a JS-callable surface, verified by a Node.js smoke harness since `syscall/js` code cannot run under the plain Go toolchain) exposing enough of the simulation loop for a `mode-*` game app to drive a match. Finish with fixture-driven integration tests: scripted, real-character-vs-character scenarios (real `.cns`/`.zss`/`.cmd`/`.air`/`.sff` data from `character`, real stage data from `stage`) driven through the full simulation loop end-to-end, asserting on match outcome, not just individual subsystem behavior. "Done" means at least one full scripted match between two real characters runs to a correct win condition through this loop alone.

## Acceptance Criteria
- [ ] KO win condition triggers correctly when a fighter's health reaches zero mid-round
- [ ] Timeout win condition triggers correctly when the round timer expires, with the documented tiebreak rule applied when both fighters have equal/nonzero health
- [ ] Round reset restores both fighters to their starting state/position/health, and match-level state correctly tracks rounds won per side across a best-of-N match
- [ ] A degenerate scripted scenario (e.g. both fighters reach zero health on the same tick, a double KO) resolves to a documented, non-crashing outcome
- [ ] A fixture-driven integration test runs a full scripted match between two real characters end-to-end through this loop and asserts the correct match outcome; the WASM build is verified by a Node.js smoke harness mirroring `character/cmd/wasm`'s approach

## Notes
Cross-repo: integration tests need real character data (via `character`) and real stage boundary/camera data (via `stage`) as fixtures — confirm both repos expose what's needed before starting this item.
