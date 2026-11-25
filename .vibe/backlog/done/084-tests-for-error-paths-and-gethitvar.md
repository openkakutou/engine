---
status: done
---
# Tests for the untested error paths, GetHitVar keys and the corpus scenarios

## Description
Found by the audit of 2026-11-21 (medium; suite run: 937 tests pass, statemachine at 76.1 % statement coverage).
- About 35 error returns for malformed `.zss` text (`zssexec/parser.go`) and about 45 unevaluable-parameter branches in `statemachine` (`gethit.go`, `pause.go`, `projectile.go`, `target.go`, `hitoverride.go`, `bind.go`, `reversal.go`) have no test; `movement_test.go` shows the pattern.
- `GetHitVar` keys `hitcount`, `damage`, `fall.xvel`, `fall.damage`, `fall.kill`, `fall.envshake.*`, `down.recovertime`, `hitid`, `chainid`, `guarded` are never evaluated by a test.
- Nine end-to-end tests need the local real-file corpus and skip without it, so CI never runs the helper, BindToTarget, Trans and projectile-trade scenarios. Decided: keep the corpus tests local, and give each feature one scenario on the committed reference characters (item 053 owns those characters).
- `TestE2E_RyuSurvivesARandomMatch` logs its `unsupported` set and never asserts it.

## Acceptance Criteria
- [x] Table tests cover the malformed `.zss` errors and the controllers' unevaluable parameters, asserting the message
- [x] Each `GetHitVar` key is read back from a distinct value
- [x] Helper, BindToTarget, Trans and projectile trade each have one end-to-end scenario that runs in CI
- [x] The random-match test asserts its `unsupported` set against a pinned list

## Done
- `statemachine/unevaluable_params_test.go`: 80 controller parameters, each given "1 +", must fail naming the controller and the parameter. It found that every list parameter and SuperPause's `anim` returned the bare evaluator error; they now go through `evalPart` (23 cases red before the fix).
- `zssexec/parser_test.go`: 24 more malformed `.zss` cases in the bad-bodies table, one per parser error not covered.
- `evaluator/gethitvar_keys_test.go`: every numeric key read back from a value no other field holds, every boolean key set alone.
- `e2e_reference_features_test.go`: helper, BindToTarget, Trans and projectile trade each run in CI on the committed kfm given a few states of the test's own and one animation, until item 053's characters cover them; the corpus scenarios stay local.
- The random Ryu match asserts its unsupported set against the pinned debug clipboard controllers.
v2.77.14.
