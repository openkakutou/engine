---
status: todo
---
# Hit Eligibility and Defence Controllers

## Description
Support NotHitBy, HitBy, DefenceMulSet and AssertSpecial. Common1 uses them to make a falling or lying fighter unhittable, and to adjust the damage it takes.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`), grounded in its source (commit `54d4148a`).

## Acceptance Criteria
- [ ] NotHitBy and HitBy filter incoming hits by attribute and time, as Ikemen GO does, with tests
- [ ] DefenceMulSet scales the damage taken
- [ ] AssertSpecial flags that common1 uses are applied or explicitly reported, each one cited

## Notes
Split out of item 035 (Product Owner, 2026-11-18): the reference files are the `common1.cns` and `common1.cns.zss` shipped with Ikemen GO (MIT, roadmap decision `037`); acceptance characters are Ikemen GO's `kfm` and `kfm_zss`.
