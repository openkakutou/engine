---
status: todo
---
# Typed names instead of silent string keys

## Description
Found by the audit of 2026-11-21 (low, item 085). About 20 `Asserted.Has("<name>")` literals, about 30 `Consts.Lookup("data.…")` / `movement.…` keys, `postype` kept as a string, and bare `StateType` / `MoveType` / `Physics` letters (`'A'`, `'H'`…): a typo reads false or 0 with no error. Typed constants (an `AssertFlag` type with the known flags, named constants keys, `StateType` / `MoveType` / `Physics` constants) let the compiler catch it.

## Acceptance Criteria
- [ ] AssertSpecial flags, constants keys and the state, move and physics letters are named once, the code reading them uses the names
- [ ] A test fails when a flag or key used by the code is not one Ikemen GO knows (src commit cited)
