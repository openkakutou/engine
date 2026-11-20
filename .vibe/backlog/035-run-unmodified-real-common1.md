---
status: todo
depends_on: [027, 028, 034, 039, 040, 041, 042, 043, 047, 048, 049, 050, 051, 052, 054]
---
# Run an unmodified real common1 end to end

## Description
`game` loads the common states from its `common/` folder (game#038, roadmap decision `031`) and hands them to the engine. Today the get-hit tests use a trimmed `common1_gethit.cns` limited to what the evaluator supports (item 028's notes). Make an unmodified MUGEN 1.1 `common1.cns` and Ikemen GO `common1.cns.zss` run: every state they define loads, and a match goes through stand, walk, jump, guard, get hit, fall, lie down and get up with them.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO, validated against the real files; any divergence between the two recorded in an ADR.

## Acceptance Criteria
- [ ] Both real common files load with no unsupported controller or trigger left
- [ ] Integration test: a real character goes through the common states above with each file
- [ ] The trimmed fixture is retired or kept only for unit tests

## Notes
Combat-completeness milestone, step order in roadmap decision `036`.
Split on 2026-11-18 (Product Owner): the real files are missing 17 controller types, about 40 triggers and part of the `.zss` grammar, so the prerequisites became items 047-052, plus 039-043. Reference files: the `common1.cns` and `common1.cns.zss` shipped with Ikemen GO (MIT), not Elecbyte's. Acceptance characters: Ikemen GO's `kfm` and `kfm_zss`, committed as fixtures (047).
Revised on 2026-11-18 (roadmap decision `044`): the acceptance characters are Ikemen GO's `kfm` and `kfm_zss`, and the common files are MUGEN 1.1's `common1.cns` and Ikemen GO's `common1.cns.zss`. All are committed as fixtures (047).
- 2026-11-21: a HitDef now stays active until it hits (Ikemen GO hitActive / hitdefTargets), the engine gets a lying fighter up (5110 -> 5120 after down.recovertime, data.liedown.time by default) and MugenVersion() is read: `e2e_fall_test.go` runs kfm's sweep on kfm (MUGEN common1.cns) and on kfm_zss (Ikemen GO common1.cns.zss), the defender going through get-hit, fall, bounce, lying down, getting up and back to 0 with control. Left: guard (item 023), walk/jump in the same scenario, retiring the trimmed fixture.
- 2026-11-21: guard done (item 023, e2e_guard_test.go with both common1). Left: retiring the trimmed get-hit fixture.
