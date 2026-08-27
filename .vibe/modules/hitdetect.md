# Module: hitdetect
**Role:** Resolves Clsn (collision box) overlap between two fighters each simulation tick: an attacker's Clsn1 (hit) boxes against a defender's Clsn2 (hurt) boxes, sourced from each fighter's currently active `character/air` animation frame. Checks both attack directions and reports one `HitEvent` per overlapping box pair. Does not apply damage itself, only detects and reports collisions — see `.vibe/decisions/009` for the local-to-world coordinate transform and the zero-allocation-on-a-zero-hit-tick discipline this hot path holds to.
**Files:** `hitdetect/hitdetect.go`
**Exports:** `Detect(state *match.MatchState, frames [2]air.Frame) []HitEvent`, `HitEvent`
**Depends on:** `modules/match.md` (`MatchState`/`FighterState`/`Side`/`Facing`/`Position`), `github.com/openkakutou/character/air` (`Frame`/`ClsnBox`, external module, read-only)
