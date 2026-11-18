---
date: 2026-11-18
status: accepted
---
# Const(), float variables and the "Other" triggers (item 042, first slice)

**Context:** kfm and kfm_zss read their own constants (`Const(movement.yaccel)`), use `fvar`/`sysfvar`, and the `.zss` state -1 opens with `!aiLevelF`. The reference is Ikemen GO, commit `54d4148a` (roadmap decision 038: no MUGEN check; where item 042 says "MUGEN timing", Ikemen GO's is followed).
**Decision:**
- **Constants table:** `evaluator.Constants`, built once per character by `ParseConstants` from the `.def` "cns" file (`kfm.cns`, `kfm.const`) over Ikemen GO's defaults (`char.go:8120-8201`, file read `:4372-4631`). `Context.Consts` holds a pointer to it; a nil pointer reads the defaults. `FighterProgram.Constants` is set by `LoadFighterProgram` and copied into the runtime by `NewRuntime`. Item 043 reads the same table through `Constants.Lookup`.
- **Names:** every name of `compiler.go:2246-2517` (138) has a value. `Const240p/480p/720p/1080p(x)` return `x` scaled from a 320/640/1280/1920-wide screen (`char.go:5746`).
- **Unknown name:** reads int 0, not an error. Ikemen GO compiles an unknown name to a lookup in the `[Constants]` section (`compiler.go:2518-2521`, the `Error("Invalid data")` is commented out) that reads int 0 when absent (`bytecode.go:3358-3360`, `1469-1475`). Only a wrong shape (`Const()`, `Const(1)`, two arguments) is an error.
- **Float variables:** `Context.FVars [40]float32`, `SysFVars [5]float32` (float32 like `char.go` `cnsfvar`). `VarSet`, `:=` and the `FVar`/`SysFVar` triggers share `Context.AssignVar`. `ErrFloatVarUnsupported` is removed.
- **Other triggers:** `AILevel`/`AILevelF` are 0 (a human player, `bytecode.go:2510-2515`, `3654-3659`, `char.go:5529-5537`); `PalNo` is `Context.PalNo`, 1 when unset (`bytecode.go:2671`, `char.go:4888-4913`); `NumHelper`/`NumExplod` (bare or with an id, `compiler.go:3415-3425`) are 0 until helpers and explods exist (items 029, 052).
- **Not done here:** the `Time` redirections (`p2, time`) belong to the opponent slice; `VarAdd` has no controller yet (item 051).
**Divergences from Ikemen GO:**

| Subject | Ikemen GO | OpenKakutou | Why |
|---|---|---|---|
| Variable index range | unbounded sparse maps, only a negative index warns (`char.go:8248-8404`) | `var` 0-59, `sysvar` 0-4, `fvar` 0-39, `sysfvar` 0-4, else an error | MUGEN's limits, fixed arrays keep `Context` allocation-free (already the rule for `var`) |
| `Const(...)` scaling | every default and `Const240p` scaled by the character's `localcoord` (`char.go:8123`, `5746`) | `localcoord` taken as 320 | `character` does not expose the `.def` `[Info] localcoord`; fix with the first non-320 character |
| String `[Constants]` | typed `string` value | skipped, reads 0 | no trigger compares strings yet |
| `crouch.friction.threshold` default | 0 (`char.go:543`) | 0 | MUGEN's documentation says .05; Ikemen GO's value is followed |
| `AILevel` | `sys.aiLevel[playerNo]`, set by the game for CPU players | always 0 | every fighter is human-driven |
| Missing "cns" file | load error | reported in `missing`, defaults apply | same handling as a missing state file |

**Reason:** one table owned by `evaluator` serves `Const()` now and physics later without per-tick allocation: a lookup is a map read on a lower-cased name already in the parsed expression.
**Rejected alternatives:** *A struct with one field per constant*: 138 fields and a switch to read them by name, for no gain over a map read once per use. *Erroring on an unknown `Const` name*: Ikemen GO accepts it and characters use `[Constants]` custom names.
