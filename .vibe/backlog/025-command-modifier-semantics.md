---
status: todo
depends_on: [028]
---
# True semantics for .cmd modifier prefixes (~, $, /, >)

## Description
Decision 006 deliberately recognised the command-string modifiers without implementing them. Real characters rely on release detection (~), 4-way direction ($), hold (/) and strict timing (>). Implement their semantics and find real .cmd files where the simplification gives a wrong match or non-match.

## Acceptance Criteria
- [ ] ~ release, / hold, $ direction guarantee and > strict one-tick timing each have distinct tested behaviour
- [ ] A corpus test over real .cmd files lists commands whose match result changes versus the old simplification
- [ ] Decision 006 is superseded by a new decision recording the final semantics

## Notes
Combat-completeness milestone, step order in roadmap decision `036`.

## Progress
- 2026-11-21 (v2.75.0): "$" done as Ikemen GO (input.go:901-960): a step's directions held whatever else is, so `/$F` holds on UF and DF. Left: "~" release detection and ">" strict timing.
