---
status: todo
depends_on: [030, 037]
---
# Core Trigger Library

## Description
The evaluator knows nine triggers (Time, StateNo, Anim, AnimTime, Ctrl, Command, var, sysvar, IfElse). Real CNS/ZSS files use many more: Vel, Pos, Life, Power, StateType, MoveType, Random, Facing, Alive, P2Dist, P2BodyDist, P2StateNo, MoveContact/MoveHit/MoveGuarded, HitDefAttr, AnimElem, plus the common math functions (abs, floor, ceil, sin, cos...).

## Acceptance Criteria
- [ ] Each listed trigger evaluates against live match state and is covered by a test
- [ ] AnimElem and the MoveContact family follow MUGEN timing semantics, with a documented Ikemen GO divergence table
- [ ] Random is deterministic from a seed carried in match state so a match replays identically
- [ ] An unknown trigger still produces a clear error naming it

## Notes
Grouped as one item because they share evaluator plumbing; split by family if the diff gets too large. Prerequisite for a meaningful result in item 030. Compat is a MUST: MUGEN 1.0/1.1 and Ikemen GO, validated against real community files.

Added while framing item 022 (hit pause and hit reaction): real `common1` get-hit states (5000-series) also need `trigger1`/`trigger2` combined as OR (today AND, decision 003), `GetHitVar` beyond hittime/slidetime/hitshaketime/animtype/groundtype/airtype (e.g. `yvel`, `fall`), `HitFall`, `CanRecover`, `SelfAnimExist`, `StateType`/`MoveType`, `Pos Y`, `Vel Y`. Item 022's fixture is a trimmed copy limited to what the evaluator supports; once these exist, an unmodified real common1 should run.
Combat-completeness milestone, step order in roadmap decision `036`.
