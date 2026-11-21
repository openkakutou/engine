---
status: done
---
# Fix the command matcher crash when two commands share a name

## Description
Real `.cmd` files declare several `[Command]` blocks with the same name as alternatives (e.g. `~D, F, x` and `~D, DF, F, x`). `input.Step` keys its progress by name, so a shorter command reads the longer one's step index and panics (index out of range); reproduced on Arale and by the harness test `TestRun_EnginePanic_IsRecordedAsTickErrorAndNextCharacterStillRuns`. 7 real characters crash with the same panic message. Each alternative must keep its own progress, and the name is active when any alternative matches, as in Ikemen GO. Ground the behavior in Ikemen GO's source (commit `54d4148a`).

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`).

## Acceptance Criteria
- [x] A test reproducing the crash with two same-name commands of different lengths passes
- [x] Either alternative activates the command name
- [x] No panic on the 7 characters that crashed in the conformance run

## Notes
Found by the conformance harness (item 030, `docs/conformance.md`, 2026-11-17).

## Done
2026-11-21 - engine v2.47.0: progress kept per [Command] block (input.State.progress by position), the name active when any alternative matches; TestStep_SameNameAlternativesKeepTheirOwnProgress reproduces the crash; the corpus characters that crashed (Necro, Fiona Belli) run.
