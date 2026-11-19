---
status: todo
---
# Player Push

## Description
The two fighters walk through each other: the fidelity video after engine#040 shows them overlapping. Ikemen GO pushes fighters apart when their size boxes overlap and both have `playerpush` set (char.go push handling, commit `54d4148a`), using `size.ground.front/back`, `size.air.*`, `size.height` and the `PlayerPush` controller.

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`).

## Acceptance Criteria
- [ ] Two fighters walking into each other stop at the distance Ikemen GO gives, with the source cited
- [ ] A `PlayerPush` controller set to 0 lets them overlap
- [ ] End-to-end: Kung Fu Man walking into Kung Fu Man stays outside its opponent's size box

## Notes
Found by the fidelity video after engine#040 (2026-11-19).
