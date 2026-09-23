---
date: 2026-09-23
status: accepted
---
# Power/meter resets to zero, forced by match.NewMatchState

**Context:** Backlog item 019 adds a power/meter value to `match.FighterState`, required to start at 0 both at match start and at every round reset — unlike `Health`, which is always whatever value the caller's starting `FighterState` supplies for that round.

**Decision:** `match.NewMatchState` unconditionally zeroes `Power` on both fighters it builds, regardless of what value the caller-supplied `FighterState.Power` held. Since both match creation (`cmd/wasm`'s `newMatch`) and every round reset (`round.ResetRound`) build their `MatchState` through this one constructor, this single change gives both required reset points the same guarantee without duplicating the rule in each caller. The power cap itself (`statemachine.DefaultMaxPower = 3000`) stays a hardcoded package constant, not a configurable parameter — `engine` has no lifebar/system-config data flowing into it at all yet (its only cross-repo inputs are `character` and `stage`), so a per-match override hook would be speculative ahead of a real second data source for it.

**Reason:** Centralizing the invariant in the one constructor already responsible for a `MatchState`'s validity (see `.vibe/decisions/001`) is more robust than trusting every call site to remember to zero it, and it makes the acceptance criterion mechanically true regardless of what a caller (or malicious/careless JSON request) supplies.

**Rejected alternatives:** Passing power through unchanged like `Health` and relying on `mode-*` callers to always send `power: 0` on `newMatch`/`resetRound` requests — rejected because it is not verifiable by this repo's own tests and silently breaks if a caller ever echoes back a previous response's `power` value into the next request. A configurable max-power override sourced from lifebar data — rejected for now since no lifebar data reaches `engine`; revisit once such a data source actually exists (matches this repo's own precedent of deferring generality until a second real use case exists, e.g. backlog item 012's own deferral).
