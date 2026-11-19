---
status: todo
---
# New State Runs on the Change Tick

## Description
In Ikemen GO, a ChangeState enters the new state and runs it in the same tick (changeStateEx loop, char.go:6984-6994, commit `54d4148a`). The engine runs a newly entered state on the next tick, so every state change is one tick late. Item 044 worked around it for landing (state 52). Make the current state's ChangeState run the new state in the same tick, as Ikemen GO does, including its loop limit (`MaxLoop`) and its hit-pause rules.

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`).

## Acceptance Criteria
- [ ] A ChangeState runs the target state's controllers in the same tick, with the loop bounded as in Ikemen GO, with the source cited
- [ ] The landing workaround of item 044 is removed: state 52's own controllers do the job
- [ ] Expected values that change are listed with the Ikemen GO reason; the end-to-end scenarios stay green

## Notes
Found by item 044 (2026-11-19). Check against the differential traces once item 036 exists.
