---
status: todo
depends_on: [027, 028, 034, 039, 040, 041, 042, 043, 047, 048, 049, 050, 051, 052]
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
