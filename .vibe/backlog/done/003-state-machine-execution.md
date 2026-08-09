---
status: done
depends_on: [002]
---
# State Machine Execution

## Description
Drive a fighter's state transitions by interpreting `character/cns`'s `StateDef`/`Controller` data using the trigger evaluator from item 002. Each simulation tick: evaluate the current state's controllers in order against `MatchState`, apply any state controller whose triggers pass (state change, variable assignment, and the small set of controller types needed to prove out the execution loop — not full MUGEN controller-type coverage, which can grow via later items), and advance to a new state when a `ChangeState`-equivalent controller fires. "Done" for this item means a fighter can be driven from one real character's `.cns` (e.g. its standing-idle state) through at least one real state transition purely by this execution loop, matching the trigger-evaluated outcome, not a hardcoded transition.

## Acceptance Criteria
- [x] Given a `StateDef` and a `MatchState`, controllers whose triggers evaluate true are applied and controllers whose triggers evaluate false are skipped, in declared order
- [x] A state change controller transitions the fighter's current state number and resets per-state execution position correctly
- [x] Running the loop against a real character's `.cns` idle state produces the expected controller applications for a hand-built `MatchState` fixture
- [x] A `StateDef`/`Controller` referencing a state number that doesn't exist in the loaded character returns a descriptive error instead of crashing or silently no-op'ing
- [x] Unit tests cover at least one multi-controller state where trigger order determines which controllers actually apply

## Notes
None.
