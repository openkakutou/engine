---
status: todo
---
# Fix gofmt Drift In `wasm-lang-spike` Benchmark

## Description
`gofmt -l .` currently flags `benchmarks/wasm-lang-spike/go/main.go` — a misaligned `const` block and a misaligned struct field, pre-existing and unrelated to any active feature work. Discovered while running the Definition-of-Done lint check for backlog item 005 (Physics And Movement); left untouched there since the drift predates and is out of scope for that item.

## Acceptance Criteria
- [ ] `gofmt -l .` reports no files across the whole `engine` repo
- [ ] `benchmarks/wasm-lang-spike/go/main.go`'s behavior is unchanged (formatting-only fix)

## Notes
Fix: `gofmt -w benchmarks/wasm-lang-spike/go/main.go`, then confirm `gofmt -l .` reports nothing. No test coverage needed — this is a formatting-only change to a benchmark spike, not application code.
