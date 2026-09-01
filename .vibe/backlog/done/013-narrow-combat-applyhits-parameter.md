---
status: done
---
# Narrow Combat ApplyHits Parameter

## Description
`combat.ApplyHits` takes a full `cns.Controller` as its `hitDef` parameter but only ever reads `Parameters["damage"]` — the SOLID review agent flagged this as an interface-segregation violation. Narrowing it would reduce `combat`'s coupling to the full `Controller` shape it doesn't need.

## Acceptance Criteria
- [x] `combat.ApplyHits` accepts only the damage string (or a small `DamageParams` type) instead of the full `cns.Controller`
- [x] `tick.go`'s call site is updated to extract `Parameters["damage"]` before calling `ApplyHits`
- [x] `combat_test.go` and `tick_test.go` are updated accordingly and pass (`tick_test.go` needed no changes — it drives `Tick` end-to-end, never calls `ApplyHits` directly)

## Notes
Finding from `/vibe:review` (SOLID agent), deferred at review time (2026-08-31, commit `019027a`) since every other call site in this codebase passes `cns.Controller` whole (`statemachine.ApplyController`, `zssexec`'s `ctrlStmt`) — weigh consistency with that convention against the narrower interface before implementing.
