---
status: todo
---
# .zss Grammar Gaps Found in kfm_zss

## Description
Three forms in Ikemen GO's `kfm_zss` still fail to parse:
- a controller parameter whose value is a list, such as `reversalDef{ reversal.attr: SA, AA; … }` (Statedefs 1300, 1320, 1340);
- a redirection with a comma inside `let`, such as `let nojug = p2,anim = 5120 || …` (AI.zss state -1);
- a negative state defined in several `.zss` files, where only the first is kept. Ikemen GO appends them (compiler.go:8466), so kfm_zss's AI -1 is dropped.

🔒 MUST: Ikemen GO behaviour, grounded in its source (commit `54d4148a`).

## Acceptance Criteria
- [ ] Every block of `kfm_zss` parses with no error
- [ ] A negative state spread over several files runs every part, in Ikemen GO's order
- [ ] Each form has a test citing the Ikemen GO source

## Notes
Found by item 054 (2026-11-18). See decision 024.
