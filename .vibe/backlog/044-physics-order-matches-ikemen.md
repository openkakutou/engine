---
status: todo
---
# Physics, gravity and hit detection in Ikemen GO's order

## Description
In the engine, `physics.Step` runs after hit detection, and adds gravity before integrating the position. Ikemen GO integrates the position before collision and adds gravity after moving (`src/char.go` posUpdate and the tick order in `src/system.go`). A VelSet therefore moves a fighter one tick later than in Ikemen GO, and jump arcs differ slightly. Align the order, citing Ikemen GO, and update the expected values that change.

🔒 MUST: Ikemen GO behaviour, verified against its source and, once available, the differential traces (item 036).

## Acceptance Criteria
- [ ] Position update, gravity and hit detection run in Ikemen GO's order within a tick, cited
- [ ] A VelSet takes effect on the position on the same tick as in Ikemen GO

- [ ] A physics-A fighter that touches the ground enters state 52 (landing), as Ikemen GO does (char.go:12584-12590), so a jump returns to state 0

## Notes
Raised by item 027 (2026-11-18).
Landing into state 52 added by item 054 (2026-11-18): the end-to-end jump scenarios fail on it.
