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
