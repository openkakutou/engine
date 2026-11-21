---
status: todo
---
# Fuzz And Malformed Input Hardening

## Description
The engine is driven by untrusted community files and runs in browsers via WASM. Add Go fuzz targets for the evaluator (expression parsing), Tick with hostile character data, and the WASM request decoding, and bound memory and time on pathological input.

## Acceptance Criteria
- [ ] Fuzz targets exist for expression evaluation, Tick and WASM request JSON, with seed corpora
- [ ] Pathological input (deeply nested expressions, huge state counts, cyclic ChangeState) returns an error or stops within a bounded step budget, never hangs or panics
- [ ] Each crash found is fixed with a regression test

## Notes
Sibling items: character 062, sff 008, snd 009, stage 017.
- [ ] A `.zss` step has a work budget, not only a call-depth limit: functions F0..F63 each calling the next twice (about 2^64 statements, `zssexec/zssexec.go:443-474`) stop with an error (audit 2026-11-21)
- [ ] The expression parser and `Eval` have a nesting limit: a trigger of 1 MB of `(` or `!` returns an error instead of exhausting the Go stack (`evaluator/parser.go:238-247, 340-350`) (audit 2026-11-21)
- [ ] Live projectiles per fighter are capped as in Ikemen GO (`ProjectileMax`, char.go:7852-7855), with its warning (`projectile.go:63-77`) (audit 2026-11-21)
