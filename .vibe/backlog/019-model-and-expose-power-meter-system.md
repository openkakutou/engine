---
status: todo
---
# Model And Expose Power/Meter System

## Description
`engine` currently has no power/super-meter concept anywhere: not modeled in any Go type, not computed by `Tick`, not present in the WASM `tick`/`newMatch`/`resetRound` JSON contract (confirmed by a repo-wide search for "power"/"meter" turning up zero matches outside unrelated identifiers). `character/cns` already parses `PowerAdd` as a state controller (type + params, uninterpreted), but `statemachine.ApplyController`'s switch only executes `ChangeState`/`VarSet` — a `PowerAdd` controller in a loaded character's states is silently a no-op today. This is what `mode-quick-versus` backlog item `004` (In-Match HUD) has been blocked on since 2026-08-31: its acceptance criterion "Power bar reflects `engine`'s live power/meter value" can't be implemented against what `engine` currently exposes.

## Acceptance Criteria
- [ ] A fighter's power/meter value is tracked in `match.FighterState`, starts at 0 at match start and at each round reset (mirroring how `Health` already resets)
- [ ] `statemachine.ApplyController` executes `PowerAdd` (increments the fighter's power/meter by its parameter), added as a new case alongside the existing `ChangeState`/`VarSet`
- [ ] Power/meter is clamped to a maximum (MUGEN's own default cap of 3000, unless the loaded character/lifebar data specifies otherwise) and never goes negative
- [ ] `round.Progress`/the WASM `tick`/`newMatch`/`resetRound` JSON contract exposes each fighter's current power/meter value, the same way `Health` is already exposed
- [ ] Existing `statemachine`/`round`/`wasm` test suites pass unchanged, plus new tests covering `PowerAdd` execution and clamping

## Notes
Cross-repo: this is the named blocker for `mode-quick-versus` backlog item `004` (In-Match HUD) — re-run that item once this ships. Also worth noting for whoever picks this up: adding `PowerAdd` as a real second controller type is the exact condition backlog item `012` (Registry Based Dispatch For Controller Types) has been waiting on since its 2026-08-31 deferral — consider revisiting `012` alongside this item, though it isn't a prerequisite for either.
