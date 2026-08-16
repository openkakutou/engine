---
status: in_progress
depends_on: [003]
---
# Input Reading And Command Matching

## Description
Read raw per-tick input (directions + button presses, from whatever host/game-mode-supplied input source `engine` accepts as a parameter — this item defines that minimal input shape, not a full input-device abstraction) and match it, with a buffering time window, against `.cmd` command definitions parsed by `character/cmd` (that repo's backlog item 036). A matched command becomes available to state-machine execution (item 003) the same way MUGEN's `Command` trigger works, so CNS triggers referencing a command name resolve correctly during evaluation. "Done" means a real character's `.cmd` motion (e.g. a quarter-circle-forward + punch special move input) is correctly recognized from a scripted sequence of raw per-tick inputs, including when the input sequence is close but not an exact match (must correctly not match).

## Acceptance Criteria
- [ ] Raw per-tick input state (directions, buttons) is modeled and accepted by the matcher across successive ticks
- [ ] A `.cmd` command's input sequence, within its declared buffer/time window, is recognized as matched when the exact sequence occurs
- [ ] An input sequence that almost but doesn't fully match a command's definition is correctly not reported as a match
- [ ] A command whose buffer window has elapsed before the full sequence completes is not matched (the required time-based edge case)
- [ ] Matched commands are queryable by state-machine execution (item 003) in a way that lets a `Command` CNS trigger resolve correctly

## Notes
Cross-repo: needs `character`'s `cmd` package (its backlog item 036) already implemented and available to import — check its status before starting.
