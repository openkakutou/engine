---
status: todo
depends_on: [001]
---
# Physics And Movement

## Description
Implement per-tick physics for a fighter: velocity integration, gravity while airborne, ground/air state transitions (landing, leaving the ground), and clamping fighter position against the current stage's boundaries. Stage boundary and camera data come from the `stage` repo (read-only input, per CLAUDE.md's cross-repo dependency note) — this item is the first place `engine` actually depends on `stage`'s output, not just `character`'s. "Done" means a fighter's position updates correctly tick-over-tick under gravity and horizontal movement, transitions between ground/air state at the right moments, and never leaves the stage's defined boundary regardless of how much velocity would otherwise carry it past the edge.

## Acceptance Criteria
- [ ] Velocity integrates into position every tick, with gravity applied while airborne and not while grounded
- [ ] A fighter transitions from airborne to grounded when landing (position crosses the ground plane) and the transition zeroes/adjusts vertical velocity as MUGEN physics expects
- [ ] Horizontal position is clamped to the stage's boundary data (from `stage`) so a fighter can never move outside it, even with a single large velocity step that would otherwise overshoot
- [ ] Physics update on a fighter with no stage boundary data supplied returns a descriptive error or falls back to a documented default, rather than panicking
- [ ] Unit tests cover a full jump arc (leaves ground, apex, lands) and a boundary-clamping case

## Notes
Cross-repo: needs `stage`'s boundary/camera data (read-only). Confirm `stage`'s current API surface for exposing this before implementation.

Per the roadmap's `.vibe/decisions/014`, `stage` may by then also expose a Z-axis extension of `StageBoundaries` (Ikemen GO 3D model-based stages). If so, confirm whether/how it applies here against `stage`'s API surface at implementation time — this item doesn't commit to Z-axis clamping today, just flags it so the information isn't lost.
