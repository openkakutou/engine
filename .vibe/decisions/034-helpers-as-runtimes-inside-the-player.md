---
date: 2026-11-21
status: accepted
---
# Helpers live inside their player's runtime and are read through snapshots

**Context:** Item 029 adds helpers (Ikemen GO helper chars). `Tick` is built around exactly two fighters (`[2]FighterRuntime`, `match.MatchState` with two `FighterState`s), and triggers reach other fighters only through value snapshots (`evaluator.Opponent`, decisions 009 and 016).

**Decision:** A helper is a `Helper` (a `FighterRuntime` plus serials) in its player's `FighterRuntime.Helpers`, run by the same `tickFighter` as a player, after both players, P1's first (Ikemen GO charList creation order), new helpers in the tick they are made. Parent, root and the player's helpers are given to a helper as `Opponent` snapshots (now also carrying vars, name and helper id), so `parent,`, `root,`, `helper(id),` and `NumHelper` read them without pointers. A helper's projectiles join its player's list; its HitDef lands like the player's, the helper taking the contact and the hit pause. `Tick` clones the helper and projectile slices it changes, so the runtimes handed in stay untouched.

**Reason:** Keeps `Tick`'s two-player shape, the match state and the WASM contract unchanged, reuses every per-fighter rule (states, physics, hit pause, HitDef arming), and keeps the snapshot discipline that avoids per-tick allocations for the common case of no helpers.

**Rejected alternatives:** A flat list of every entity in `MatchState` (Ikemen GO's charList) rewrites `Tick`, the round flow and the WASM state for every caller at once. Pointers from a helper to its parent and root make `Context` escape through the controller registry (decision 016).

Shortcuts, marked in code: helpers cannot be hit, pushed or screen-bound; keyctrl, postype front/back/left/right; a helper into a missing state is skipped with a warning instead of entering an empty state.
