---
status: todo
---
# Free Acceptance Character

## Description
Author an OpenKakutou test character under a free licence, in a `.cns` version and a `.zss` version. It replaces Kung Fu Man, which belongs to Elecbyte and cannot be redistributed, as the acceptance character of the combat-completeness milestone. It must run in Ikemen GO as well, so the differential tests (item 036) can compare against it.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility. The character runs unmodified in Ikemen GO (roadmap decision `038`).

## Acceptance Criteria
- [ ] The character has `.def`, `.air`, `.cmd`, `.cns`, a `.zss` variant and a minimal `.sff`, all committed under a free licence
- [ ] It covers what common1 exercises: stand, walk, crouch, jump, a few attacks that can be guarded, a throw and a projectile
- [ ] It loads through `character` and starts a match in Ikemen GO without error

## Notes
Product Owner decision, 2026-11-18: author our own instead of using Kung Fu Man (Elecbyte, non-commercial terms). See roadmap decision `043`.
