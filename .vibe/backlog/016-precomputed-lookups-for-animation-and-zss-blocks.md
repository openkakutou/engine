---
status: todo
---
# Precomputed Lookups For Animation And Zss Blocks

## Description
The performance review agent noted that `tick.findAnimation` does a linear scan of a fighter's full animation list every tick to find the current animation, and `zssexec.findStatedef`/`findFunction` similarly rescan a script's blocks on every lookup — both operate on data that is loaded once and static for a match. Precomputing an index would turn these into O(1) lookups.

## Acceptance Criteria
- [ ] `tick.go`'s `FighterProgram` gains a precomputed `map[int]air.Animation` (or equivalent), built once, and `tickFighter` looks up by `Anim` number directly instead of scanning
- [ ] `zssexec`'s Statedef/Function blocks are indexed once (e.g. by state number / function name) instead of rescanned on every `findStatedef`/`findFunction` call
- [ ] All existing tests pass unchanged; no observable behavior change

## Notes
Finding from `/vibe:review`'s performance agent (2026-08-31, commit `019027a`). Lower urgency than the expression/step/body caching fixes already applied in that review: `zssexec` is not currently wired into `Tick` (see `.vibe/decisions/011`), so this is not yet on the real hot path for that package; `tick.findAnimation` is on the real hot path but with typically small animation-list sizes.
