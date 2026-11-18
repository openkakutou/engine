---
status: todo
---
# VarRangeSet and VarAdd Controllers

## Description
Support VarRangeSet and VarAdd for int and float variables, as common1 uses them.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`), grounded in its source (commit `54d4148a`).

## Acceptance Criteria
- [ ] VarAdd adds to var, fvar, sysvar and sysfvar, in every parameter form Ikemen GO accepts
- [ ] VarRangeSet sets a range of variables, with the default first and last indices taken from Ikemen GO
- [ ] An invalid index is a clear error

## Notes
Split out of item 035 (Product Owner, 2026-11-18): the reference files are the `common1.cns` and `common1.cns.zss` shipped with Ikemen GO (MIT, roadmap decision `037`); acceptance characters are Ikemen GO's `kfm` and `kfm_zss`. Related to 039 (VarSet's `var(N) = value` form).
