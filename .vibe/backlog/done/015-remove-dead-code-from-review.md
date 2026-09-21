---
status: done
---
# Remove Dead Code From Review

## Description
The hygiene review agent flagged a handful of dead-code/leftover items: an unused `tokenKinds` helper in `evaluator/lexer_test.go`, dead stores in `input/matcher_test.go` (flagged by `staticcheck` SA4006), and four unused opcodes (`opSub`, `opMul`, `opLT`, `opAnd`) in the `wasm-lang-spike` benchmark's bytecode VM.

## Acceptance Criteria
- [x] `tokenKinds` is removed from `evaluator/lexer_test.go`
- [x] The dead-store intermediate assignments in `input/matcher_test.go`'s `TestStep_SequenceCompletedAfterTheBufferWindowElapses_NeverRecognizesTheCommand` use `_` instead of unused variables
- [x] The four unused opcodes in `benchmarks/wasm-lang-spike/go/main.go` are either removed or exercised by `buildPrograms` (removed)
- [x] `go vet ./...` (and `staticcheck`, if available) report no new issues; all tests pass (staticcheck not installed in this environment)

## Notes
Low-severity findings from `/vibe:review`'s hygiene agent (2026-08-31, commit `019027a`), not auto-fixed per the review skill's own rule.
