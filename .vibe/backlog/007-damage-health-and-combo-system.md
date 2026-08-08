---
status: todo
depends_on: [006]
---
# Damage/Health And Combo System

## Description
Apply the hit events produced by item 006 to actual match state: damage subtracted from the defender's health, combo counter incremented while hits land within the combo window and reset when it lapses or the defender's state changes to a recovered/non-hit state, and a defender transitioned into an appropriate hit-reaction state. This is where `HitDef`-style attack parameters (already available as raw `Controller` parameters from `character/cns`, or from `.zss`-driven state per item 004) actually get interpreted numerically for the first time in the backlog — damage value, guard behavior at a basic level if in scope, and combo scaling if the character data provides it. "Done" means a scripted sequence of real hits against a real character's health value produces the expected remaining health and combo count, including a hit that would take health below zero.

## Acceptance Criteria
- [ ] A hit event reduces the defender's health by the attack's declared damage value
- [ ] Consecutive hits within the combo window increment a combo counter; a gap beyond the window (or a defender state reset) resets it to zero
- [ ] Health does not go below zero (or whatever the documented floor is) when a hit's damage would otherwise overshoot it — this is the required error/edge case
- [ ] A hit event referencing a `HitDef` with missing/malformed damage data falls back to a documented default rather than crashing the simulation
- [ ] Unit tests cover a multi-hit combo sequence and a lethal/overkill hit

## Notes
None.
