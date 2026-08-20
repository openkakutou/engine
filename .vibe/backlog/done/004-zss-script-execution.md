---
status: done
depends_on: [002]
---
# `.zss` Script Execution

## Description
Execute Ikemen GO's `.zss` state scripts — the Lua-like alternative to classic `.cns` state definitions — parsed by `character`'s `zss` package (backlog item 037 in that repo) into a structured document model with script bodies kept as unevaluated raw text, per `roadmap`'s `.vibe/decisions/012`. This item makes those scripts actually run: most likely by embedding a Lua-compatible interpreter, which would be `engine`'s first third-party dependency — but the specific interpreter (or a hand-written minimal-compatible subset, if a full Lua implementation proves disproportionate) is deliberately left to be chosen during this item's own implementation, not decided up front. The executed scripts must be able to read and mutate `MatchState`/fighter state (item 001) the same way CNS-driven state execution does (item 003), since a character uses `.cns` or `.zss`, never both, and both paths must produce equivalent simulation behavior. "Done" means a real Ikemen GO character's `.zss` state script runs against a `MatchState` fixture and produces the expected state/variable changes, not just a synthetic script.

## Acceptance Criteria
- [ ] A chosen interpretation strategy (embedded Lua-compatible interpreter or hand-written subset) is documented as an explicit decision, including why
- [ ] `.zss` script bodies parsed by `character/zss` execute and can read fighter/match state
- [ ] Script execution can mutate fighter/match state (e.g. trigger a state change) with the mutation visible to the rest of the simulation loop
- [ ] A script referencing an unsupported Lua/Ikemen construct returns a descriptive error rather than crashing the simulation
- [ ] Fixture-driven tests execute real `.zss` scripts sourced from actual Ikemen GO characters, not only hand-written scripts

## Notes
Cross-repo: depends on `character`'s `zss` package (its backlog item 037) already being implemented and available to import — check its status before starting.
