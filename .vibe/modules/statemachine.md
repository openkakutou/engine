# Module: statemachine
**Role:** Drives a fighter's state transitions by interpreting `character/cns`'s `StateDef`/`Controller` data with `engine/evaluator`, one simulation tick at a time.
**Files:** `statemachine/statemachine.go`
**Exports:** `Step(ctx evaluator.Context, states map[int]cns.StateDef) (Result, error)`, `Result{Context evaluator.Context, Applied []int}`, `ApplyController(ctrl cns.Controller, ctx *evaluator.Context, exists func(int) bool) (bool, error)`, `ControllerTypeChangeState`/`ControllerTypeVarSet`/`ControllerTypeHitDef` (string constants — the single source of truth for controller-type names other packages, e.g. the root `engine` package's own `HitDef` classification in `tick.go`, need to recognize)
**Depends on:** `modules/evaluator.md`, `modules/match.md` (via `evaluator.Context`), `character/cns` (external module, `StateDef`/`Controller` — read-only)
**Depended on by:** `modules/zssexec.md` (reuses `ApplyController` to apply `.zss` controller-shaped statements)
