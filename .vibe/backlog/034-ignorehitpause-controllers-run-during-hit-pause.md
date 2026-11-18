---
status: done
depends_on: [022]
---
# Controllers flagged ignorehitpause run during a hit pause

## Description
Item 022 freezes a fighter during a hit pause by skipping its state machine entirely. MUGEN and Ikemen GO instead skip only the controllers *without* `ignorehitpause = 1` (MUGEN 1.1 cns doc: "will not be checked during a hit pause"); real characters rely on it for hit sparks, sounds and similar effects that must keep running while frozen (Ikemen's common1.cns.zss wraps such blocks in `ignoreHitPause{}`).

## Acceptance Criteria
- [x] During a hit pause, a controller with `ignorehitpause = 1` is still evaluated and applied; every other controller is skipped, as today
- [x] The `.zss` `ignoreHitPause{}` block form gets the same behavior, or the gap is recorded in an ADR (gap recorded in `.vibe/decisions/021`)
- [x] No per-tick heap allocation added on the frozen path (decisions 009/010/011)

## Notes
Deferred from item 022 by the Product Owner. Must stay compatible with MUGEN 1.0/1.1 and Ikemen GO.
Combat-completeness milestone, step order in roadmap decision `036`.
