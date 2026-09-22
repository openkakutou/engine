---
status: todo
---
# Expose Sound-Trigger Events (PlaySnd)

## Description
`character/cns` already parses `PlaySnd` as a state controller (type + params, uninterpreted), but `statemachine.ApplyController`'s switch has no case for it — a `PlaySnd` controller in a loaded character's states is silently a no-op today. `engine` should trigger the event, not decode or play any audio itself, matching its existing simulation-not-rendering scope (roadmap `.vibe/decisions/004`, `008`).

## Acceptance Criteria
- [ ] `statemachine.ApplyController` executes `PlaySnd` by recording a triggered `(group, sample)` pair for that fighter this tick, as a new case alongside `ChangeState`/`VarSet`
- [ ] The WASM `tick` JSON contract exposes each tick's triggered sound events (which fighter, which group/sample), the same shape as it already exposes animation state
- [ ] A tick with no `PlaySnd` controller executed returns an empty events list, not `null` (matching this repo's existing JSON-normalization guarantee)
- [ ] Existing `statemachine`/`wasm` test suites pass unchanged, plus new tests covering `PlaySnd` triggering and the empty-tick case

## Notes
Raised while scoping org-wide audio support, see roadmap `.vibe/decisions/026`. No dependency on the new `snd` repo — `engine` only reports which group/sample index was triggered; decoding and playing the actual audio is `mode-quick-versus`'s job (its own `#013`), same split already established for `character`'s own state/animation data vs. its rendering. Same "generic controller parsed, nothing executes it yet" pattern as `#019` (Power/Meter); worth picking up together if convenient, not required.
