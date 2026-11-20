---
status: done
depends_on: [054]
---
# VarRangeSet and VarAdd Controllers

## Description
Support VarRangeSet and VarAdd for int and float variables, as common1 uses them.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`), grounded in its source (commit `54d4148a`).

## Acceptance Criteria
- [x] VarAdd adds to var, fvar, sysvar and sysfvar, in every parameter form Ikemen GO accepts
- [x] VarRangeSet sets a range of variables, with the default first and last indices taken from Ikemen GO
- [x] An invalid index is a clear error (VarAdd); an invalid VarRangeSet range sets nothing, silently, as Ikemen GO does (char.go:8425-8427)

## Notes
Split out of item 035 (Product Owner, 2026-11-18): the reference files are the `common1.cns` and `common1.cns.zss` shipped with Ikemen GO (MIT, roadmap decision `037`); acceptance characters are Ikemen GO's `kfm` and `kfm_zss`. Related to 039 (VarSet's `var(N) = value` form).
- 2026-11-21: done. VarAdd shares VarSet's forms; VarRangeSet's defaults are first 0, last 59 (bytecode.go:12031-12032).
