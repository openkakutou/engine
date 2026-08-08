---
status: todo
---
# Define Match/Combat State Model

## Description
Define the pure-data model representing the live state of a match while two characters fight: per-fighter state (position, facing, velocity placeholder, current animation/state reference, health/round placeholders as needed by later items) plus match-level state (round number, round timer, which side each fighter is on). This is the foundation every later `engine` package (evaluator, state machine, physics, hit detection, damage, round flow) reads from and writes to — get its shape right now, since retrofitting it later touches everything downstream. No evaluation, execution, or simulation logic lives here — this item is the data model only, matching `character`'s established pattern of separating pure-data types from the logic that operates on them.

## Acceptance Criteria
- [ ] A `MatchState` (or equivalently named) struct models round/timer and holds two fighters' state
- [ ] A per-fighter state type models position, facing, and the identifying fields later items (evaluator, physics, state machine) will need to reference
- [ ] Zero-value structs are valid and usable (e.g. a zero-value fighter state doesn't panic when read)
- [ ] Invalid construction (e.g. a match state built with fewer than two fighters, if the API allows constructing one) is handled explicitly rather than left to panic downstream
- [ ] Unit tests cover the zero-value case and basic field access for both the fighter and match-level types

## Notes
None.
