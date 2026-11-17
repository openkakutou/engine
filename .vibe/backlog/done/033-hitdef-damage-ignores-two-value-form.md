---
status: done
---
# HitDef damage written as two values deals no damage

## Description
A HitDef written `damage = 23, 0` (hit damage, guard damage) — the normal form in real characters, e.g. Ikemen GO's bundled kfm (`kfm.cns` state 200) — deals 0 damage: `combat.parseDamage` runs `strconv.Atoi` on the whole string and falls back to `DefaultDamage` (0) when it fails. Only a single bare integer works today.

## Acceptance Criteria
- [ ] `damage = 23, 0` deals 23 on a landed hit; the second value (guard damage) is parsed and kept for guarding (item 023)
- [ ] A single value (`damage = 15`) still works; an omitted value still defaults to 0, as MUGEN documents
- [ ] A fixture-driven test uses a real character's HitDef damage values

## Notes
Found while framing item 022. MUGEN 1.1 sctrls doc: both damage values default to 0. Must stay compatible with MUGEN 1.0/1.1 and Ikemen GO.
