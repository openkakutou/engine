---
status: todo
---
# Remaining Triggers Used by Reference Characters

## Description
After item 041, every expression of the reference files parses, but some trigger names are still unknown to the evaluator:
- edge distances: `BackEdgeBodyDist`, `BackEdgeDist`, `FrontEdgeBodyDist` (kfm.cns, hits.zss);
- `GameTime` (AI.zss);
- `InGuardDist`, `MatchOver` and `RoundsExisted` (both common files);
- `MugenVersion()` (common1.cns.zss);
- `Map()`, `Name`, `TeamSide` and `aiw` (kfm.zss);
- `StateTime` (kfm.cmd).

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`), commit `54d4148a`.

## Acceptance Criteria
- [ ] Each name evaluates as Ikemen GO does, with the source cited and a test using a value derived independently
- [ ] The conformance run on `kfm` and `kfm_zss` (with `-common testdata/reference`) lists no unknown trigger

## Notes
Found by item 041 (2026-11-19).
- 2026-11-21: edge distances, `InGuardDist` done (with `HitPauseTime`, `ID`, `JugglePoints`, `NumProj`, `StandBy`, `map()` for kfm_zss AI.zss); the conformance run on kfm and kfm_zss lists no unknown trigger. Still to do: `GameTime`, `MatchOver`, `RoundsExisted`, `MugenVersion()`, `Name`, `TeamSide`, `aiw`, `StateTime`.
- 2026-11-21: `MugenVersion()` done.
- 2026-11-21: `GameTime` (the caller's tick count), `Name` / `P1Name` / `P2Name` / `P3Name` / `P4Name` / `AuthorName` (also redirected), `NumPartner`, `GetHitVar(fallcount)` done, found by the corpus Ryu (SVC). Still to do: `MatchOver`, `RoundsExisted`, `TeamSide`, `aiw`, `StateTime`.
- 2026-11-21: `StateTime` done. Still to do: `MatchOver`, `RoundsExisted`, `TeamSide`, `aiw`.
- 2026-11-21: `TeamSide` done. Still to do: `MatchOver`, `RoundsExisted`, `aiw`.
