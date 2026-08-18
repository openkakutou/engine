# Module: engine/physics
**Role:** Advances a fighter's position and vertical velocity by one simulation tick — gravity while airborne, landing on the ground plane, and clamping horizontal position to the current stage's boundaries.
**Files:** `physics/physics.go`
**Exports:** `Step(f match.FighterState, bounds *stage.StageBoundaries, gravity float64) (match.FighterState, error)`
**Depends on:** `modules/match.md`, `github.com/openkakutou/stage` (external, tagged module — `StageBoundaries`)
