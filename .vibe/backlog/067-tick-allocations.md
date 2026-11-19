---
status: todo
---
# Remove the per-tick heap allocations of a match step

## Description
`game` item 063 measured `testing.AllocsPerRun` on one match step through `Tick` (engine v2.24.0, `game/native/internal/core`): 6 heap allocations per tick. This repo's own convention (`.vibe/index.md`, decisions 009-011) is no per-tick allocation on the common path, since the WASM build runs GC synchronously inside the frame. Find the 6 with `go build -gcflags=-m` and an allocation profile on a reference-fixture match, and remove them.

## Acceptance Criteria
- [ ] A test pins `testing.AllocsPerRun` of a reference-fixture match step at 0 (or names each remaining allocation and why it cannot go)
- [ ] No behavior change: end-to-end scenarios unchanged
