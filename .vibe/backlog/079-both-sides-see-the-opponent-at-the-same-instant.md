---
status: todo
---
# Both sides' helpers and explods see the opponent at the same instant

## Description
Found by the audit of 2026-11-21 (medium). `p1Seen` is taken at `tick.go:511`, before P2's Target controllers act on P1 (520-523). P1's helpers and explods get a fresh `p2Seen` (537), but P2's helpers (542) and explods (548) get the stale snapshot of P1: an Explod or Helper with `postype = p2` made by P2 during a TargetBind throw is placed from P1's position before the bind.

## Acceptance Criteria
- [ ] The snapshot of P1 given to P2's helpers and explods is taken after P2's target operations
- [ ] A test binds P1 with TargetBind and places a `postype = p2` explod in the same tick, from P2's side, at the bound position
