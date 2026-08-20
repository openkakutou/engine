# Module: zssexec
**Role:** Executes `character/zss`'s parsed Statedef/Function `.zss` script bodies against a fighter's live `evaluator.Context` -- the `.zss` counterpart to `statemachine.Step` for `.cns`-driven state execution.
**Files:** `zssexec/zssexec.go`, `zssexec/parser.go`
**Exports:** `Step(ctx evaluator.Context, script zss.Script) (Result, error)`, `Result{Context evaluator.Context}`
**Depends on:** `modules/evaluator.md` (condition evaluation), `modules/statemachine.md` (reuses `ApplyController` for controller-shaped statements), `character/zss` (external module, `Script`/`Block` -- read-only)
