# Module: engine/physics
**Role:** Ikemen GO's `posUpdate` for one fighter and one tick: position += velocity, horizontal position clamped to the current stage's boundaries, then gravity subtracted from the vertical velocity. No ground stop: the landing (state 52) is `Tick`'s. See `.vibe/decisions/029`. Player push, which also moves fighters and clamps them to the same bounds, is `Tick`'s (`push.go`, decision 031).
**Files:** `physics/physics.go`
**Exports:** `Step(f match.FighterState, bounds *stage.StageBoundaries, gravity float64) (match.FighterState, error)`
**Depends on:** `modules/match.md`, `github.com/openkakutou/stage` (external, tagged module — `StageBoundaries`)
