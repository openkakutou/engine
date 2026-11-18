---
status: todo
---
# Run the special states -3, -2 and -1 every tick

## Description
MUGEN and Ikemen GO run the special states -3, -2 and -1 every tick before the fighter's current state; characters put their command-driven state changes in -1. `Tick` runs only the current state (`tick.go:325`), so in the conformance run only 4 of 442 characters ever left their first state. Run them in the reference order and with the reference rules (which apply to helpers, what happens when one changes state). Ground the behavior in Ikemen GO's source (commit `54d4148a`). Combat-completeness milestone: this comes first, before 027/028 (roadmap decision `036`'s measure-first order).

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`).

## Acceptance Criteria
- [x] A command in state -1 moves the fighter into its special move state
- [x] Order and rules match Ikemen GO's source, cited
- [ ] Conformance run: most characters reach more than one state (moved to 039/041/028: their -1 triggers need them, see docs/conformance.md)

## Notes
Found by the conformance harness (item 030, `docs/conformance.md`, 2026-11-17).

## Done
Special states run in `tick.go` through `statemachine.StepState`, after Ikemen GO `char.go:12497-12531` (actionRun) and `bytecode.go:5749-5775` (ChangeState stops only its own state). Limits left: helpers, HitDef inside special states, same-tick run of a state entered from the current state, ChangeState to the same state number.
