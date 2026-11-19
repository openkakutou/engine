---
status: done
---
# A change to a state that does not exist continues as Ikemen GO does

## Description
`statemachine` returns an error ("ChangeState targets state N, which does not exist in the loaded character"), which stops the whole match (found by `game` 064 in the browser: a character whose `.cns` changes to state 10 with no `common1.cns` loaded). Ikemen GO (`src/char.go:6880-6887` @54d4148a) warns "changed to invalid state N (from state M)" and enters an empty state whose state type, move type and physics are "unchanged"; the match goes on. Reproduce that (roadmap decision `041`), for `.cns` and `.zss` changes alike, and surface the warning to the caller instead of an error.

🔒 MUST: as Ikemen GO.

## Acceptance Criteria
- [x] A ChangeState (and any other state change: SelfState, a hit's p2stateno, `.zss` changes) to a missing state enters an empty state with unchanged state type, move type and physics, as Ikemen GO; no error
- [x] The warning is reported to the caller (Tick result), naming the target and the previous state
- [x] An end-to-end scenario through `Tick` on the reference fixtures pins it
