---
status: todo
---
# Public Character State Loader with Search Dirs

## Description
The game builds a fighter from the `.def`'s `cns` file only. It ignores `st`, `stN`, `stcommon` and every `.zss` file, so no Kung Fu Man match can start in the game: state 0 lives in common1, as the fidelity video of 2026-11-18 showed. engine's conformance harness already loads all of them the way Ikemen GO does (`buildProgram`, decision 024). Make that loader public, so the game and the harness share one implementation. Add the search directories Ikemen GO uses for a state file the character folder does not hold (`SearchFile`: the `.def` folder, then the game's own folders such as `data/`; src/compiler.go:8793, src/common.go:758).

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`), commit `54d4148a`.

## Acceptance Criteria
- [ ] A public engine function builds a `FighterProgram` from a loaded character and its `.def` path. It follows Ikemen GO's file order and first-definition-wins rule, and searches extra directories after the character folder
- [ ] With the reference folder as the extra directory, `kfm` (with `common1.cns`) and `kfm_zss` (with `common1.cns.zss`) both get state 0 and the common states
- [ ] The conformance harness uses it. Its existing tests stay green
- [ ] A missing state file, or a `.zss` that fails to parse, gives the same result as today: skipped, or an error naming the file

## Notes
Needed by game#053. Found by the fidelity video (2026-11-18).
