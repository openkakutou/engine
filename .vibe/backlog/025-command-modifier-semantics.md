---
status: todo
---
# True semantics for .cmd modifier prefixes (~, $, /, >)

## Description
Decision 006 deliberately recognised the command-string modifiers without implementing them. Real characters rely on release detection (~), 4-way direction ($), hold (/) and strict timing (>). Implement their semantics and find real .cmd files where the simplification gives a wrong match or non-match.

## Acceptance Criteria
- [ ] ~ release, / hold, $ direction guarantee and > strict one-tick timing each have distinct tested behaviour
- [ ] A corpus test over real .cmd files lists commands whose match result changes versus the old simplification
- [ ] Decision 006 is superseded by a new decision recording the final semantics
