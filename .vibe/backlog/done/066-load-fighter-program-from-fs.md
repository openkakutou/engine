---
status: done
depends_on: []
---
# Load a fighter program from an `fs.FS`

## Description
Roadmap decision `045` (`game` item 062): `LoadFighterProgram` opens state files (`.cns`, `.zss`, `stcommon`) by path in the def's folder and the search folders. Add `LoadFighterProgramFS(fsys fs.FS, ch, defName string, searchDirs ...string)` with names inside `fsys`, doing exactly the same search (def folder first, then search folders in order, `.zss` fallback, case-insensitive fallback, Ikemen GO's rules of decision `041`). `LoadFighterProgram` is built on it, so there is one implementation. Requires `character` 084 only for the matching `LoadFS` in tests.

🔒 MUST: no change for any program `LoadFighterProgram` builds today.

## Acceptance Criteria
- [x] `LoadFighterProgramFS` over an `fstest.MapFS` builds the same program as `LoadFighterProgram` on disk for the reference fixtures
- [x] Search order, `.zss` fallback and case-insensitive lookup behave identically inside an `fs.FS`
- [x] `LoadFighterProgram` is implemented on top of it; existing tests pass unchanged
