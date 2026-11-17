---
status: todo
---
# Support PlaySnd's `S` and `F` prefixed sound groups

## Description
`PlaySnd value = S5, 0` plays from the common sound file (`S` prefix in Ikemen GO; `F` for fightfx sounds in MUGEN). The engine evaluates `S5` as a trigger: in the conformance run, 11 characters use such groups (reported as unknown triggers like `s4000`). Report the event with its source (character or common) so `game` plays it from `common/` (roadmap decision `031`). Ground the behavior in Ikemen GO's source (commit `54d4148a`).

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`).

## Acceptance Criteria
- [ ] `S`/`F` prefixed groups produce a sound event flagged as common
- [ ] Unprefixed groups unchanged

## Notes
Found by the conformance harness (item 030, `docs/conformance.md`, 2026-11-17).
