---
status: todo
---
# Fix the command matcher crash when two commands share a name

## Description
Real `.cmd` files declare several `[Command]` blocks with the same name as alternatives (e.g. `~D, F, x` and `~D, DF, F, x`). `input.Step` keys its progress by name, so a shorter command reads the longer one's step index and panics (index out of range); reproduced on Arale and by the harness test `TestRun_EnginePanic_IsRecordedAsTickErrorAndNextCharacterStillRuns`. 7 real characters crash with the same panic message. Each alternative must keep its own progress, and the name is active when any alternative matches, as in Ikemen GO. Ground the behavior in Ikemen GO's source (commit `54d4148a`).

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`).

## Acceptance Criteria
- [ ] A test reproducing the crash with two same-name commands of different lengths passes
- [ ] Either alternative activates the command name
- [ ] No panic on the 7 characters that crashed in the conformance run

## Notes
Found by the conformance harness (item 030, `docs/conformance.md`, 2026-11-17).
