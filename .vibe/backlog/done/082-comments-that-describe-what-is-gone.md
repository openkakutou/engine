---
status: done
---
# Comments that deny what the engine now does

## Description
Found by the audit of 2026-11-21 (medium). Comments contradict the code next to them:
- "not yet" notes on things that exist: `tick.go:1408` (no guard, state 120 never entered), `evaluator/context.go:311` (MoveGuarded / MoveReversed read 0), `evaluator/opponent.go:104` (no helpers), `statemachine/statemachine.go:337` ("the two types this package implements today") and `:405` (no custom states), and the stale `shortcut:` notes at `projectile.go:84`, `explod.go:47`, `round/clock.go:79`.
- "HitDef is the first one" at `tick.go:188` and `zssexec/zssexec.go:57`; the last one that ran wins.
- Doc comments stacked on the wrong declaration: `tickFighter`'s above `inGuardState` (`tick.go:735`), `gravityFor`'s above `applyFriction` (`tick.go:2053`), `evalGetHitVar`'s (`evaluator/eval.go:448`), `AttackMul`'s on `Bind` (`evaluator/context.go:273`).

## Acceptance Criteria
- [x] Each listed comment says what the code does, sits on its own declaration, or is removed
- [x] The `shortcut:` notes name only what is still not done

## Done
Each comment checked against the code (and Ikemen GO where it cites it). The `shortcut:` notes now name only what is still missing: in `basicActions`, back still counting against forward within guard distance, the lying-fighter check of the guard entry, the air jump and the round-over jump lock; projectile heightbound and depth; explod friction and binding to another fighter; the KO slow motion. `redirectKinds` now names `stateowner` as the one redirection left unanswered. `gravityFor`'s comment was moved with item 083. v2.77.13.
