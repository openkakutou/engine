---
status: todo
---
# The WASM session counts a round's result once

## Description
Found by the audit of 2026-11-21 (medium). `tickJS` (`cmd/wasm/main.go:439`) adds the round result to the session's progress on every tick whose outcome is not none, and `CheckOutcome` keeps reporting the KO while a fighter's health is 0. A host that ticks on to show the KO before `resetRound` (the doc invites it) gives the winner one round win per tick: a best-of-3 ends after one KO. `game` plays through its own core, which refuses such ticks, so only a direct user of `cmd/wasm` is hit.

## Acceptance Criteria
- [ ] The session records a round's result once; `resetRound` clears it
- [ ] `smoke.mjs` ticks past a KO and checks the wins stay at 1
