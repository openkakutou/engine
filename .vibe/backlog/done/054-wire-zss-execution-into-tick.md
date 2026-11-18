---
status: done
---
# Wire .zss Execution into Tick

## Description
`.zss` states run only in zssexec's own tests: `Tick` never calls zssexec (decision 011 scoped it out), so no match runs a `.zss` character or Ikemen GO's `common1.cns.zss`. Wire it in, so that a fighter whose states come from `.zss` plays a match through `Tick` like a `.cns` fighter. The end-to-end rule in `CLAUDE.md` then applies to `.zss` too. Do this before items 049-052, so that their controllers are tested end to end on both paths.

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`), grounded in its source (commit `54d4148a`).

## Acceptance Criteria
- [x] A fighter program can carry a compiled `.zss` script. `Tick` runs its current state, and states -3/-2/-1 when defined in `.zss`, in Ikemen GO's order, with `.cns` and `.zss` states mixed as Ikemen GO allows (cited)
- [x] During a hit pause, `.zss` `ignoreHitPause` blocks run as Ikemen GO does, closing the gap of decisions 021 and 023
- [x] Skipped calls and unsupported elements of `.zss` reach `TickResult` and the WASM response
- [x] End-to-end scenario: `kfm_zss` with `common1.cns.zss` plays a scripted match through `Tick` (stand, walk, jump). Steps that need a missing trigger are marked as expected failures, each naming the item that will make it pass

## Notes
Product Owner decision, 2026-11-18: end-to-end tests as we go (`CLAUDE.md`, "End-to-end tests").
