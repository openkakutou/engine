---
status: todo
depends_on: [030, 037]
---
# Movement And State Controllers

## Description
Implement the controllers every character relies on to move and change state: VelSet, VelAdd, VelMul, PosSet, PosAdd, ChangeAnim, CtrlSet, StateTypeSet, MoveTypeSet, Turn, LifeAdd, LifeSet, Gravity-related physics flags, Null. Unsupported types must still fail loudly, not silently do nothing.

## Acceptance Criteria
- [ ] A state with VelSet or PosAdd changes the fighter's velocity/position on the tick its triggers hold
- [ ] ChangeAnim and Turn change the animation number and facing observably in the tick result
- [ ] LifeAdd/LifeSet clamp health to [0, max] and never revive a KO'd fighter by accident
- [ ] A controller type still unimplemented is reported by name, not applied as a no-op
- [ ] Every controller above has a test and an Ikemen GO .zss equivalent where one exists

## Notes
Engine currently implements only ChangeState, VarSet, PowerAdd and PlaySnd (docs/architecture.md), so real characters cannot yet play out. Item 012 (registry dispatch) is blocked: prefer building on its registry, or extend the existing RegisterController map. Feeds roadmap 013 (combat-completeness milestone). Compat is a MUST: MUGEN 1.0/1.1 and Ikemen GO, validated against real community files.
Combat-completeness milestone, step order in roadmap decision `036`.
