---
status: todo
depends_on: [030, 037]
---
# Differential conformance against Ikemen GO

## Description
Roadmap decision `036`: fidelity during a fight is the first goal. The conformance harness (030) counts unsupported controllers and triggers; it cannot tell whether supported ones behave like the reference. Record, from Ikemen GO, the per-tick state (state number, animation and element, position, velocity, life, power, hit/guard flags) of both fighters for a scripted input sequence on the acceptance characters, and compare the engine's run tick by tick, reporting the first divergence.

The replay is the shared basis of game#051 (full-match states and screenshots): inputs of both players recorded frame by frame, both engines stepped one frame at a time with no wall clock, and the same random seed, so `Random` cannot cause false divergences.

**No CPU**: both sides are driven by simulated inputs only (a scripted or recorded input sequence per player); the CPU opponent is never used, since its decisions are not the reference engine's and would make the two runs diverge for reasons unrelated to compatibility (Product Owner, 2026-11-17).

First establish feasibility, including whether Ikemen GO can be given a fixed random seed (driving Ikemen GO with recorded inputs and dumping its state, e.g. through its debug or Lua hooks); if it is not feasible, record why in an ADR and fall back to hand-checked reference traces.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified through Ikemen GO as the reference (roadmap decision `038`): no separate MUGEN check. Only characters that work in Ikemen GO are compared.

## Acceptance Criteria
- [ ] A reference trace exists for each acceptance character's scripted sequence
- [ ] The engine run is compared tick by tick, first divergence reported with both values
- [ ] Runs in CI on the recorded traces

## Notes
Combat-completeness milestone, step order in roadmap decision `036`.
