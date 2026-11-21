---
status: done
---
# HitDef Parameters Evaluated When It Runs

## Description
Ikemen GO evaluates every HitDef parameter when the controller runs, and each HitDef that runs replaces the previous one: the last one wins (bytecode.go:8631-8650, commit `54d4148a`). In the engine, a parameter written as an expression (e.g. `damage = var(1)` or `pausetime = 10 + random % 3`) falls back to its default, because `combat.ParseHitDef` reads literals only (decision 018). When several HitDefs run in one tick, the first one wins. Applies to `.cns` and `.zss` alike (decision 030).

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`).

## Acceptance Criteria
- [x] Every HitDef parameter Ikemen GO evaluates is evaluated when the HitDef runs, with the fighter's context, in both formats
- [x] When several HitDefs run in one tick, the last one wins, as in Ikemen GO
- [x] A warm tick in which a literal HitDef lands adds no allocation (AllocsPerRun)

## Notes
Found by item 062 (2026-11-19).

## Done
2026-11-21 - engine v2.38.0: the number parameters (`hitDefNumbers`) are evaluated when the HitDef runs, list parts split with `evaluator.SplitList` (parentheses and redirection commas kept), decimals truncated as `evalI`; the last HitDef run in a tick wins in both formats; a literal HitDef allocates nothing (`TestResolveHitDef_ALiteralHitDefAllocatesNothing`). Found blocking the corpus Ryu (SVC)'s hadouken (`damage = floor(90*root,fvar(0)*root,fvar(3)),10`).
