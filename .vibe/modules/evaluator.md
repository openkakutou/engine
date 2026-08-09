# Module: evaluator
**Role:** Parses and evaluates MUGEN CNS trigger/expression strings — the raw `Controller.Triggers`/`Controller.Parameters` strings `character/cns` deliberately leaves unevaluated — against a fighter's live runtime state, returning a loosely-typed (bool/int/float) result or a descriptive error.
**Files:** `evaluator/lexer.go`, `evaluator/ast.go`, `evaluator/parser.go`, `evaluator/context.go`, `evaluator/eval.go`
**Exports:** `Parse(expr string) (*Expression, error)`, `(*Expression).Eval(ctx Context) (Value, error)`, `Evaluate(expr string, ctx Context) (Value, error)`, `Context` (embeds `match.FighterState`; adds `Time`, `Ctrl`, `Anim`, `AnimTime`, `Vars [60]int`, `SysVars [5]int`, `ActiveCommands map[string]bool`), `Value` (`Kind` + `Number float64`, with `Bool()`/`Float()`/`Int()`), `Kind` (`KindBool`, `KindInt`, `KindFloat`)
**Depends on:** `modules/match.md` (embeds `match.FighterState` in `Context`)
