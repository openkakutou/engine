---
date: 2026-08-09
status: accepted
---
# Match state holds fighters in a fixed two-slot array, built through a validating constructor

**Context:** Defining the pure-data `MatchState`/`FighterState` model (backlog item 001) — the foundation every later `engine` package (evaluator, state machine, physics, hit detection, damage/combo, round flow) reads from and writes to. The acceptance criteria require both that a zero-value `MatchState` be safely readable and that invalid construction (fewer than two fighters) be handled explicitly rather than left to panic downstream.

**Decision:** `MatchState.Fighters` is a fixed `[2]FighterState` array indexed by a `Side` enum (`SideP1 = 0`, `SideP2 = 1`), not a slice or a map. Direct struct literals (`MatchState{}`, `FighterState{}`) remain valid and panic-free. A separate constructor, `NewMatchState(round, roundTimer int, fighters ...FighterState)`, accepts fighters positionally-independent (matched by each `FighterState.Side`, not by argument order) and returns a descriptive error instead of a malformed `MatchState` when `fighters` does not contain exactly one `SideP1` and one `SideP2` entry.

**Reason:** A fixed two-element array matches the domain directly (a fighting match is always two-sided) and keeps `Fighters[side]` a cheap, panic-free array index for the hot per-tick read path later packages will use. Routing construction through `NewMatchState` — rather than only exposing the array — gives the "invalid construction is handled explicitly" acceptance criterion a real, testable error path (wrong fighter count, duplicate `Side`) instead of that check being vacuously satisfied by the array's fixed size alone.

**Rejected alternatives:**
- **`[]FighterState` slice:** allows a `MatchState` with 0, 1, or 3+ fighters to exist as a bare struct literal with no constructor involved at all, pushing the "exactly two" invariant onto every consumer instead of centralizing it.
- **`map[Side]FighterState`:** same variable-length problem as a slice, plus map access requires a comma-ok check or an extra existence assumption everywhere a fighter is read — worse ergonomics than a fixed array for a hot per-tick path, for no flexibility this domain needs.
- **Two named fields (`P1`, `P2 FighterState`) instead of an indexed array:** considered for readability, but later packages (evaluator, physics) frequently need "the other fighter" or "the fighter for this `Side`" generically — an indexed array makes that a plain expression (`m.Fighters[side]`) instead of a switch on which named field to use.
