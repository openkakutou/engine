---
status: todo
---
# Round-Start Ctrl and Statedef Header Defaults

## Description
At round start Ikemen GO gives every fighter control (`ctrl = 1`, system.go:3364). The engine instead takes the start state's header `ctrl`, which is false when omitted. Entering a state whose header omits `ctrl` resets Ctrl to false here, while Ikemen GO leaves it unchanged. The header's `velset` and `poweradd` are not applied on entry, and header fields written as expressions are not evaluated, and an omitted `anim` becomes the state number where Ikemen GO keeps the current animation. This affects both `.cns` and `.zss` states.

🔒 MUST: Ikemen GO behaviour, grounded in its source (commit `54d4148a`, compiler.go `stateDef` 7020, char.go `changeState`).

## Acceptance Criteria
- [ ] Each fighter has control at round start, as in Ikemen GO
- [ ] A header with no `ctrl` leaves Ctrl unchanged on entry. `velset`, `poweradd` and expression-valued fields apply as Ikemen GO does, with the source cited
- [ ] The end-to-end scenarios no longer force `Ctrl = true` in their setup

## Notes
Found by item 054 (2026-11-18).
