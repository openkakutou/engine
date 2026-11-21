---
status: todo
depends_on: [029]
---
# Explod lifecycle

## Description
Explods are reported today as one-tick `Explod` events. Keep them as live entities like projectiles and helpers: animation, position (postype, bind), velocity and acceleration, removetime, removeongethit, ModifyExplod and RemoveExplod, NumExplod, so the game can draw them for their whole life. Split from item 029.

MUST: Ikemen GO behaviour (roadmap decision `038`), commit `54d4148a`.

## Acceptance Criteria
- [ ] Explod instances are reported per tick (animation, position, remaining time) so the game can draw them
- [ ] RemoveExplod, ModifyExplod, removetime and removeongethit behave as in Ikemen GO; NumExplod counts them
- [ ] A cap on live explods prevents unbounded growth, with a clear warning beyond it
- [ ] End-to-end scenario on a real character's explod
