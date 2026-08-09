# Module: statemachine
**Role:** Drives a fighter's state transitions by interpreting `character/cns`'s `StateDef`/`Controller` data with `engine/evaluator`, one simulation tick at a time.
**Files:** `statemachine/statemachine.go`
**Exports:** `Step(ctx evaluator.Context, states map[int]cns.StateDef) (Result, error)`, `Result{Context evaluator.Context, Applied []int}`
**Depends on:** `modules/evaluator.md`, `modules/match.md` (via `evaluator.Context`), `character/cns` (external module, `StateDef`/`Controller` — read-only)
