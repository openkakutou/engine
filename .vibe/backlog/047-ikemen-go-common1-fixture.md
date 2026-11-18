---
status: in_progress
---
# Ikemen GO Common1 Fixture

## Description
Add Ikemen GO's `data/common1.cns.zss` to the repo as a test fixture, with its provenance and the MIT licence notice. Real `.zss` content can then run in CI. MUGEN 1.1's `common1.cns` belongs to Elecbyte and cannot be redistributed, so tests use it only when a local copy is provided.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`), grounded in its source (commit `54d4148a`).

## Acceptance Criteria
- [ ] Ikemen GO's `common1.cns.zss` is committed unmodified, with its source commit and the MIT licence text beside it
- [ ] A test loads it through `character` without error
- [ ] A test loads a local MUGEN `common1.cns` when one is provided, and is skipped with a clear message otherwise
- [ ] No Elecbyte file is committed (roadmap decision `037`)

## Notes
Split out of item 035. Revised on 2026-11-18 (Product Owner): Ikemen GO ships no `common1.cns`, only the `.zss`. Its `kfm`/`kfm_zss` are Elecbyte content under non-commercial terms (Ikemen-GO-Screenpack README), so acceptance characters moved to item 053.
