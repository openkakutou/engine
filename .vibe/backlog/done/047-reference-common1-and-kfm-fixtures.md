---
status: done
---
# Reference Common1 and Kfm Fixtures

## Description
Commit the reference content as test fixtures, unmodified, so tests on real content run in CI: Ikemen GO's `data/common1.cns.zss`, MUGEN 1.1's `common1.cns`, and Kung Fu Man (`kfm` and `kfm_zss`) from the Ikemen-GO-Screenpack repo. Each fixture records its source and keeps its licence notice (roadmap decision `044`).

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`).

## Acceptance Criteria
- [x] The four fixtures are committed unmodified, with a provenance file giving the source (repo and commit, or install) and the licence of each
- [x] A test loads each common file and each character through `character` without error
- [x] A test fails if a fixture file is missing or altered (checksum recorded in the provenance file)

## Notes
Split out of item 035. Revised on 2026-11-18 (roadmap decisions `043`, then `044`).
