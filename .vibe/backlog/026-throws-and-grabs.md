---
status: todo
depends_on: [022]
---
# Throws: grab, hold and throw states

## Description
Implement throws: a HitDef with a throw/grab attribute that connects puts the defender in the held state bound to the attacker (TargetBind), and the attacker's subsequent controllers can apply damage, state changes and release. Guard must not stop a throw.

## Acceptance Criteria
- [ ] A throw HitDef connects regardless of guarding and binds the defender to the attacker
- [ ] TargetState / TargetBind style controllers move the defender during the throw
- [ ] Defender is released cleanly at the end of the throw or on a round reset
- [ ] Fixture test with a real character's command throw
