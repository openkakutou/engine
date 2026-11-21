---
status: todo
---
# Architecture docs match the engine

## Description
Found by the audit of 2026-11-21 (high). `docs/architecture.md` and `CLAUDE.md` state rules the code and the accepted decisions contradict:
- `docs/architecture.md:218` and the "CNS only" diagram edge: `.zss` is said not to run in `Tick` (decision 011); it does (`tick.go:233`, decision 024).
- `docs/architecture.md:65`: a same-tick transition is said to lag one tick; decision 033 runs the new state in the same tick.
- `docs/architecture.md:93-98`: triggers are said to be ANDed (decision 003); decision 032 ORs the groups.
- `docs/architecture.md:5, 57`: "ten sub-packages" with nine drawn (`conformance` missing); the `Tick` order ignores decisions 029, 031, 033, 034.
- `CLAUDE.md`: "Early scaffold stage — only `version.go` exists", and a Lua-compatible interpreter "anticipated" where decision 008 chose a hand-written one.
- Decision 019 says `conformance` parses and merges the state files itself; it calls `engine.LoadFighterProgram`.

## Acceptance Criteria
- [ ] `docs/architecture.md` describes `.zss` in `Tick`, the same-tick state change, the ORed trigger groups and today's `Tick` order, each citing its decision
- [ ] Its two diagrams show the ten packages and the `root → zssexec` edge (what they show: the packages and their dependencies; one tick's flow)
- [ ] `CLAUDE.md` Architecture and Stack describe the engine as it is
- [ ] A new decision supersedes 019 (and the wording of 024 point 6)
- [ ] `/vibe:sync` refreshes `.vibe/index.md` and the glossary (helper, projectile, explod, HitDef, hit pause)
