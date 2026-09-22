---
date: 2026-09-22
status: accepted
---
# Animation and `.zss` block lookups are precomputed once through a constructor, not cached lazily

**Context:** Backlog item 016 — `tick.findAnimation` linearly scans a fighter's full animation list every simulation tick, and `zssexec.findStatedef`/`findFunction` linearly rescan a script's blocks on every lookup call within a `Step` invocation. Both scan data that is loaded once per match and never changes afterward.

**Decision:** `FighterProgram` gains an unexported `map[int]air.Animation` built once by a new validating constructor, `NewFighterProgram(states, animations, commands)`, mirroring the existing `NewFighterRuntime` constructor already in this file. `tickFighter`'s lookup uses the map when present and falls back to the original linear scan when a `FighterProgram` is built via a raw struct literal (existing tests, `integration_test.go`) — so no existing test needs to change. `cmd/wasm`'s session setup, the only real production caller, is updated to rebuild each fighter's `FighterProgram` through `NewFighterProgram` once at match-creation time, before storing it in the long-lived session — the fallback path is reachable only from tests.

`zssexec` gains a `Compile(script zss.Script) CompiledScript` step that precomputes a `map[int]zss.Block` (by Statedef number) and a `map[string]zss.Block` (by Function name) in one pass. `Step` now takes a `CompiledScript` instead of a raw `zss.Script`; this is a safe, self-contained signature change since `zssexec.Step`'s only caller today is its own test file (not yet wired into `Tick`, per decision 011).

Both map-build loops implement first-occurrence-wins explicitly (skip the insert if the key is already present), matching the original linear scan's behavior on malformed/duplicate input (two animations sharing a number, two Statedef blocks sharing a number, two Functions sharing a name).

**Reason:** A constructor built once outside the tick loop is the only shape that actually removes the scan from the hot path — `tickFighter` does exactly one animation lookup per fighter per tick, so indexing lazily inside that same call would cost strictly more (build + lookup) than the scan it replaces. Following the same validating-constructor idiom already established for `FighterRuntime`/`MatchState` (decision 001) keeps the codebase's few sanctioned "build once, thread everywhere" patterns consistent instead of introducing a new one (a lazily-populated cache, an identity-keyed package-level map, or a custom `UnmarshalJSON`) for what is fundamentally the same problem already solved once.

**Rejected alternatives:**
- **Lazy, first-call caching keyed by slice/script identity (pointer or reflect-based):** would remove the scan from later calls but relies on Go slice/pointer identity surviving unchanged across a match, an unsafe assumption this codebase's existing memoization (`parseCache`, `bodyCache`) deliberately avoids by keying on stable string content instead; scripts and animation lists have no equivalent stable string key cheap to compute without already scanning them.
- **Custom `UnmarshalJSON` on `FighterProgram`:** would build the index automatically as part of `cmd/wasm`'s JSON decoding, but introduces a JSON-marshaling idiom with no precedent anywhere else in this repo, and still leaves every non-JSON caller (every existing test, `integration_test.go`) needing the same struct-literal fallback this decision already requires — no simpler in practice.
- **Changing `Animations`'s wire shape from a JSON array to an object keyed by number:** would make the field itself already-indexed after JSON decoding with no separate map, but changes `cmd/wasm`'s external request contract, which decision 012 already treats as something this repo commits to keeping additive-only.
