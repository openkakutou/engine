---
status: todo
---
# Tests for the untested error paths, GetHitVar keys and the corpus scenarios

## Description
Found by the audit of 2026-11-21 (medium; suite run: 937 tests pass, statemachine at 76.1 % statement coverage).
- About 35 error returns for malformed `.zss` text (`zssexec/parser.go`) and about 45 unevaluable-parameter branches in `statemachine` (`gethit.go`, `pause.go`, `projectile.go`, `target.go`, `hitoverride.go`, `bind.go`, `reversal.go`) have no test; `movement_test.go` shows the pattern.
- `GetHitVar` keys `hitcount`, `damage`, `fall.xvel`, `fall.damage`, `fall.kill`, `fall.envshake.*`, `down.recovertime`, `hitid`, `chainid`, `guarded` are never evaluated by a test.
- Nine end-to-end tests need the local real-file corpus and skip without it, so CI never runs the helper, BindToTarget, Trans and projectile-trade scenarios. Decided: keep the corpus tests local, and give each feature one scenario on the committed reference characters (item 053 owns those characters).
- `TestE2E_RyuSurvivesARandomMatch` logs its `unsupported` set and never asserts it.

## Acceptance Criteria
- [ ] Table tests cover the malformed `.zss` errors and the controllers' unevaluable parameters, asserting the message
- [ ] Each `GetHitVar` key is read back from a distinct value
- [ ] Helper, BindToTarget, Trans and projectile trade each have one end-to-end scenario that runs in CI
- [ ] The random-match test asserts its `unsupported` set against a pinned list
