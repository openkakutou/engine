---
status: done
---
# .zss Grammar Gaps Found in kfm_zss

## Description
Three forms in Ikemen GO's `kfm_zss` still fail to parse:
- a controller parameter whose value is a list, such as `reversalDef{ reversal.attr: SA, AA; … }` (Statedefs 1300, 1320, 1340);
- a redirection with a comma inside `let`, such as `let nojug = p2,anim = 5120 || …` (AI.zss state -1);
- a negative state defined in several `.zss` files, where only the first is kept. Ikemen GO appends them (compiler.go:8466), so kfm_zss's AI -1 is dropped.

🔒 MUST: Ikemen GO behaviour, grounded in its source (commit `54d4148a`).

## Acceptance Criteria
- [x] Every block of `kfm_zss` parses with no error
- [x] A negative state spread over several files runs every part, in Ikemen GO's order
- [x] Each form has a test citing the Ikemen GO source

## Notes
Found by item 054 (2026-11-18). See decision 024.
Item 041 (2026-11-19) confirmed both parse failures: AI.zss `let nojug = p2,anim = 5120 || …` is cut at the redirection commas by the statement splitter, and kfm.zss's `reversalDef` has an empty `reversal.attr`.
- 2026-11-21: done — redirection commas inside `let` values (splitValues), empty parameter values kept (readKeyValue), negative states appended across files (compiler.go:8462-8478). `TestE2E_KfmZssWithAnAILevel_ItsOwnAIPlaysWithoutInput` runs the AI 600 ticks.
