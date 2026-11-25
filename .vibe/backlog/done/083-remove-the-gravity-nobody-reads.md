---
status: done
---
# Remove the gravity nobody reads

## Description
Found by the audit of 2026-11-21 (medium). `tickFighter` never reads its `gravity` parameter (`tick.go:1017`): a fighter falls by its own `movement.yaccel` (`gravityFor`, `tick.go:1171`), and `TickConfig.Gravity` is documented as not read. The value is still threaded through `tickOrFreeze` (`pause.go:27, 35`) and `tickHelpers` (`helper.go:67`), the WASM `newMatch` still accepts and stores a `gravity` key (`cmd/wasm/main.go:57, 144, 239, 431`), and `conformance/conformance.go:37` still defines it. A host believes the setting has an effect.

## Acceptance Criteria
- [x] `tickFighter`, `tickOrFreeze` and `tickHelpers` no longer take gravity; `gravityFor` reads the context
- [x] The WASM session and `conformance` no longer carry it; `docs/wasm.md` says the `gravity` request key is ignored
- [x] `TickConfig.Gravity` stays only as a documented compatibility field, and its doc paragraph (`tick.go:411-417`) says so

## Done
`tickFighter`, `tickOrFreeze` and the helper tick no longer take gravity; `gravityFor` reads `ctx.YAccel` (new test `tick_gravityfor_test.go`) and its doc comment moved onto it from `applyFriction`. The WASM request and session, the conformance tool, the smoke harness, the README example and the tests' `TickConfig` literals no longer carry it; `docs/wasm.md`, `docs/architecture.md` and `docs/conformance.md` say a fighter falls by its own yaccel. v2.77.12.
