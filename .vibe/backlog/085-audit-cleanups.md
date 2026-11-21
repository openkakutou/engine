---
status: todo
---
# Audit cleanups (low)

## Description
Low findings of the audit of 2026-11-21, worth taking when the code is next touched:
- Silent string keys: 20 `Asserted.Has("<name>")` literals, about 30 `Consts.Lookup("data.…")`, `TargetOp.Kind` switched with no default, `postype` kept as a string, bare `StateType` / `MoveType` / `Physics` letters. A typo reads false or 0 with no error: typed constants would catch it.
- `1 - x.Facing` in 9 places (`Facing.Opposite()`); `ParseAttr(text, hitdef bool)` called with bare booleans; `snapshotOf` in the root package while its inverse lives in `evaluator`.
- `animationLength` (`projectile.go:274`) disagrees with Ikemen GO's total time (anim.go:444 checks only the last frame); `persistentValue` re-implements `ikemenAtoi`; `clsnWorld` copies `hitdetect.worldBox`.
- `ExplodModification.ID` shadows `ExplodSpawn.ID` and `id` is evaluated twice (a `Random` in it draws twice).
- Loader: `exists` and `resolveCaseInsensitive` report a permission error as a missing file (`loader.go:94-97, 270-273`).
- Per-tick allocations when a helper, projectile or explod is alive (`tick.go:530-535`, `helper.go:46`) and in `input.Step` (item 067).
- `getUp` mutates (rename it); `physics.go:4` says gravity is added (it is subtracted); unresolvable "this item's own ADR" references; `conformance.go:254-268` iterates a map, so the reported error is not deterministic.
- Long functions: `tickFighter` (~214 lines, 14 parameters), `applyHitReaction` (a `defer` patching `GetHit` after early returns), `parseIdentifierOrCall`.
- Backlog 012's evaluator half (trigger registry) is blocked on "a real second trigger", which now exists.

## Acceptance Criteria
- [ ] Each point is fixed or explicitly kept with its reason
