---
status: done
---
# Reduce Complexity In Evaluator And Tick

## Description
The `/vibe:review` complexity agent flagged `evaluator.parsePrimary` and `evaluator.lex` as high-complexity ("problem" severity) recursive-descent functions, and flagged `tick.Tick`'s 8-parameter signature and `input.matcher.advance`'s 5-parameter signature as reducing readability. This item extracts helper functions from the lexer/parser and bundles `tick.Tick`'s simulation-constant parameters into a config struct.

## Acceptance Criteria
- [x] `evaluator.parsePrimary` is split so its identifier/call-argument-list branch lives in its own helper function, reducing its cyclomatic complexity
- [x] `evaluator.lex` is split into `scanString`/`scanNumber`/`scanIdent` helpers, reducing its complexity and nesting depth
- [x] `tick.Tick`'s `bounds`/`gravity`/`tick`/`comboWindow` parameters are grouped into a single config struct, cutting its parameter count
- [x] All existing tests in `evaluator` and the root package pass unchanged (behavior-preserving refactor)

## Notes
Finding from `/vibe:review` (complexity agent), review commit `019027a` (2026-08-31). Purely internal refactor for the lexer/parser split — no exported behavior change expected there. `tick.Tick`'s exported signature does change if its parameters are bundled, which is a breaking API change for callers (`cmd/wasm`, and ultimately `mode-quick-versus`) — coordinate a version bump accordingly.
