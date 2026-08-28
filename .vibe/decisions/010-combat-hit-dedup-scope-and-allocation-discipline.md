---
date: 2026-08-28
status: accepted
---
# Damage/combo: one hit per attacker per call, scoped to the checked acceptance criteria, by-value state

**Context:** Backlog item 007 applies `hitdetect`'s `HitEvent`s to match state (damage, health floor, combo counter). `hitdetect`'s own decision record (009) deliberately left "what counts as one hit" open: `Detect` returns one event per overlapping Clsn box *pair*, not deduplicated. Item 007's own Description also mentions driving the defender into "an appropriate hit-reaction state" — but none of its five checked acceptance criteria actually require that (they cover damage, combo increment/reset, the health floor, and a malformed-damage fallback only). This runs twice per simulation tick (once per attacking side) for the whole match, on the same real-time, single-threaded-GC WASM target `hitdetect`'s own decision record already establishes the allocation stakes for.

**Decision:**
1. Multiple `HitEvent`s between the same (attacker, defender) pair in one call count as exactly one landed hit — MUGEN registers one hit per attack regardless of how many collision boxes overlap simultaneously. A plain linear scan for the first matching event is enough (no set/map), since a call is already scoped to one attacker.
2. State-machine-driven effects (a defender's hit-reaction state transition) are out of this item's scope — no acceptance criterion checks it, and detecting "an appropriate hit-reaction state" would require state-machine integration this item was never asked to build. A future item can add it without reopening this one.
3. `ApplyHits` takes and returns `match.MatchState` **by value**, mirroring `physics.Step`'s existing shape — a `*MatchState` return would force the whole struct to escape to the heap on every call (it's returned past the call frame), while a small fixed-size value return stays stack-allocatable. Damage is parsed via `strconv.Atoi` exactly once, after the dedup scan resolves whether a hit landed at all — never redundantly per matched event.

**Reason:** Deduping to one hit per pair matches real MUGEN behavior and keeps the combo counter meaningful (a single attack should count as a single combo hit, not scale with how many hurt boxes it happened to overlap). Narrowing scope to the checked acceptance criteria avoids building unrequested state-machine coupling now. The by-value signature and single-parse discipline follow directly from real-time-rendering consultation on this exact per-tick hot path, the same allocation-discipline reasoning `hitdetect`'s own decision record already applied.

**Rejected alternatives:**
- *One HitResult per overlapping box pair* — rejected: would let a single attack with several overlapping boxes count as multiple combo hits, wrong relative to real MUGEN behavior.
- *`*MatchState` in/out, matching `hitdetect`'s own pointer-based signature* — rejected for this function specifically: `hitdetect.Detect` only ever *reads* `*MatchState`, never returns one, so it never pays a return-escape cost; `ApplyHits` returns a new state every call, where the value-vs-pointer choice actually matters.
