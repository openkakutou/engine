---
status: done
---
# Support VarSet's `var(N) = value` parameter form

## Description
Real characters write `VarSet` as `var(3) = 1` (also `fvar(N)`, `sysvar(N)`) instead of `v = 3` + `value = 1`; the engine rejects it with a missing `v` parameter error, which stopped 21 characters in the conformance run. Ground the behavior in Ikemen GO's source (commit `54d4148a`).

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`).

## Acceptance Criteria
- [x] `var(N) = expr` sets the variable, as `v`/`value` does
- [x] Same for the other forms Ikemen GO accepts, cited
- [x] An invalid index is a clear error

## Notes
Found by the conformance harness (item 030, `docs/conformance.md`, 2026-11-17).
fvar(N)/sysfvar(N) forms need float variable storage, not in this engine yet; tracked with FVar triggers in item 042
