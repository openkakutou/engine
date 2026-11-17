---
status: todo
---
# Accept the full trigger expression grammar

## Description
The conformance run found 349,244 controllers with an unparsable trigger across 470 characters; interval comparisons (`= [a,b]`, `!= (a,b]`) are the largest identifiable cause in a sample, alongside other forms the evaluator rejects. Cover the grammar of Ikemen GO's expression compiler: intervals, redirections (`p2, StateNo`, `root,`, `helper(N),`, `target,` …), operators (`:=`, `**`, bitwise, `!`), and the trigger-specific forms (`AnimElem = N, >= 0`). Ground the behavior in Ikemen GO's source (commit `54d4148a`).

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`).

## Acceptance Criteria
- [ ] Each grammar form has a test grounded in the cited source
- [ ] Conformance run: controllers with an unparsable trigger drop to near zero, the rest listed as items

## Notes
Found by the conformance harness (item 030, `docs/conformance.md`, 2026-11-17).
