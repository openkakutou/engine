---
status: todo
depends_on: [030]
---
# Differential conformance against Ikemen GO

## Description
Roadmap decision `036`: fidelity during a fight is the first goal. The conformance harness (030) counts unsupported controllers and triggers; it cannot tell whether supported ones behave like the reference. Record, from Ikemen GO, the per-tick state (state number, animation and element, position, velocity, life, power, hit/guard flags) of both fighters for a scripted input sequence on the acceptance characters, and compare the engine's run tick by tick, reporting the first divergence.

First establish feasibility (driving Ikemen GO with recorded inputs and dumping its state, e.g. through its debug or Lua hooks); if it is not feasible, record why in an ADR and fall back to hand-checked reference traces.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO; MUGEN's closed source cannot be traced, so MUGEN-specific divergences are checked by hand and recorded.

## Acceptance Criteria
- [ ] A reference trace exists for each acceptance character's scripted sequence
- [ ] The engine run is compared tick by tick, first divergence reported with both values
- [ ] Runs in CI on the recorded traces

## Notes
Combat-completeness milestone, step order in roadmap decision `036`.
