---
status: todo
depends_on: [035]
---
# Own Reference Characters Covering Every Feature

## Description
Author OpenKakutou's own reference characters under a free licence, in a `.cns` version and a `.zss` version. Once the combat milestone is done, they replace Kung Fu Man and the Elecbyte files as the test subjects. To make good test subjects, together they must exercise every feature: each controller and trigger that MUGEN 1.0/1.1 and Ikemen GO support, plus the Ikemen GO extensions.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility. The characters run unmodified in Ikemen GO, so the differential tests (item 036) can compare against them (roadmap decision `038`).

## Acceptance Criteria
- [ ] The characters (`.def`, `.air`, `.cmd`, `.cns`, `.zss` variant, `.sff`, `.snd`) are committed under a free licence
- [ ] A coverage check lists every controller and trigger supported by Ikemen GO (source commit cited) and shows each one used at least once by the characters, with no gap left unexplained
- [ ] They load through `character` and play a full scripted match in Ikemen GO without error
- [ ] The engine's tests and differential traces use them, and the Elecbyte fixtures (roadmap decision `044`) can then be removed

## Notes
Product Owner request, 2026-11-18: to be done once the current work is finished (combat milestone, item 035). Not to be confused with the abandoned item 053 of roadmap decision `043`, which was a minimal character with the same goal.
