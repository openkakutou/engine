---
status: todo
---
# Ikemen GO Common1 and Kfm Fixtures

## Description
Commit the files shipped with Ikemen GO as test fixtures: `data/common1.cns`, `data/common1.cns.zss`, `chars/kfm` and `kfm_zss`. Each copy keeps its provenance and the MIT licence. They replace the local copies, which cannot be redistributed, so tests on real content run in CI.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`), grounded in its source (commit `54d4148a`).

## Acceptance Criteria
- [ ] The four fixtures are in the repo, each with its source, its Ikemen GO version and the MIT licence text
- [ ] A test loads each fixture through `character` without error
- [ ] No non-redistributable file is committed (roadmap decision `037`)

## Notes
Split out of item 035 (Product Owner, 2026-11-18): the reference files are the `common1.cns` and `common1.cns.zss` shipped with Ikemen GO (MIT, roadmap decision `037`); acceptance characters are Ikemen GO's `kfm` and `kfm_zss`.
