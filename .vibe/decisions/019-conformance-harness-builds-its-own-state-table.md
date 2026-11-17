---
date: 2026-11-17
status: accepted
---
# The conformance harness builds its own state table and only observes the engine

**Context:** Backlog item 030 measures, on real characters, which controllers and triggers the engine does not support yet.
**Decision:** The `conformance` package parses a character's `st`, `stN`, `.cmd` and `stcommon` files itself (`character.Load` only exposes the `cns` file's states), merging them first-definition-wins in Ikemen GO's order (`src/compiler.go`, commit `54d4148a`), with the `cns` file's states as a last fallback for MUGEN characters that keep states there. It never changes the engine: it neutralizes, on a copy, the controllers using a trigger the evaluator rejects, recovers engine panics as a recorded error, and counts exposure as ticks spent in a state using an unsupported element. Supported means registered in `statemachine` or handled by `Tick` (`HitDef`).
**Reason:** Measuring must not depend on fixing what is measured; a `character` change to expose every state file is a separate item.
**Rejected alternatives:** *A lenient mode inside `statemachine`* — would change the engine to observe it. *Counting only evaluated triggers* — needs the engine to run past them, which it cannot yet.
