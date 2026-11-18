---
date: 2026-11-18
status: accepted
---
# .zss: remaining divergences from Ikemen GO after item 048

**Context:** Item 048 brought zssexec close to Ikemen GO's `.zss` compiler and runtime (commit `54d4148a`). The review fixed four differences: the `_` parameter slot, where `persistent` counters are reset, short-circuit evaluation of controller parameters, and invalid calls being skipped instead of aborting the state. This decision records the differences that remain.
**Decision:** These differences are accepted for now:
- **Persistent counters:** they reset as soon as the state changes. Ikemen GO delays the reset during a hit pause for MUGEN characters (`char.go:6896-6903`).
- **`ignoreHitPause`:** it is recorded but not acted on (gap of decision 021).
- **Statement boundaries:** the parser does not require one statement per line at the root (Ikemen's `statementEnd`), and does not treat `:` as the end of a statement.
- **Loops and switch:** `for`, `while` and `switch` are rejected as "not supported yet". The rules on `break` and `continue` are therefore not checked.
- **`$name` check:** done by scanning the text. Ikemen GO compiles the expression, so the error messages differ on an invalid expression.
- **Parse errors:** they are collected by `Compile` and returned only when the block runs. Ikemen GO refuses the whole file at load. A call to a function that has a parse error still returns that error.
- **Duplicate functions:** the first definition wins. Ikemen GO refuses a duplicate in the same file.
- **Skipped calls:** they are reported in `Result.SkippedCalls`. Ikemen GO writes them to its console.
**Reason:** None of these differences changes how a valid Ikemen GO script runs, except the delayed reset and `ignoreHitPause`. Those two concern hit pause, and they get settled when zssexec is wired into `Tick` (decision 021).
**Rejected alternatives:** *Refusing the whole script on the first parse error*: one bad block would block every state of the character, while the engine already reports exactly which blocks are bad.
