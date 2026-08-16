# Module: input
**Role:** Reads a fighter's raw per-tick input and matches it, within a buffered time window, against `character/cmd`'s `.cmd` command definitions, producing the recognized-command set `engine/evaluator`'s `Command` trigger reads.
**Files:** `input/matcher.go`, `input/parser.go`
**Exports:** `TickInput{Up, Down, Left, Right bool, Buttons map[string]bool}`, `State` (opaque, threaded across ticks), `Step(state State, tick int, facing match.Facing, in TickInput, cmds cmd.CommandFile) (State, map[string]bool)`
**Depends on:** `modules/match.md` (`Facing`, to resolve raw Left/Right against a command's facing-relative F/B), `character/cmd` (external module, `CommandFile`/`Command` — read-only)
