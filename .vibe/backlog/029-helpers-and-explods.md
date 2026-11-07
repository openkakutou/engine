---
status: todo
---
# Helpers And Explods

## Description
Support the Helper controller (spawned sub-entities with their own state machine, owned by a parent) and Explod (animated, positioned visual effects). Most real characters use both for effects, partners' fireballs and hit sparks.

## Acceptance Criteria
- [ ] Helper spawns a fighter-like entity that runs its own states each tick and is destroyed by DestroySelf or on round reset
- [ ] Helper triggers (Parent, Root, HelperID, NumHelper) resolve correctly
- [ ] Explod instances are reported per tick (animation, position, remaining time) so the game can draw them
- [ ] A cap on live helpers and explods prevents unbounded growth, with a clear error beyond it

## Notes
Interacts with projectiles (024) and mode-quick-versus 024; keep the per-tick report format consistent with how projectiles are exposed. Match state must remain JSON-serializable across the WASM boundary. Compat is a MUST: MUGEN 1.0/1.1 and Ikemen GO, validated against real community files.
