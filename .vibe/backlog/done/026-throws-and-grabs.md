---
status: done
depends_on: [023]
---
# Throws: grab, hold and throw states

## Description
Implement throws: a HitDef with a throw/grab attribute that connects puts the defender in the held state bound to the attacker (TargetBind), and the attacker's subsequent controllers can apply damage, state changes and release. Guard must not stop a throw.

## Acceptance Criteria
- [x] A throw HitDef connects regardless of guarding and binds the defender to the attacker
- [x] TargetState / TargetBind style controllers move the defender during the throw
- [x] Defender is released cleanly at the end of the throw or on a round reset
- [x] Fixture test with a real character's command throw

## Notes
Combat-completeness milestone, step order in roadmap decision `036`.

## Done
2026-11-21 - engine v2.50.0: p1stateno / p2stateno (custom states through fighterPrograms), p1facing / p2facing, hitflag (v2.49.0), the Target* controllers applied to the target after the attacker's tick, ChangeAnim2, SelfState, GetHitVar(isbound); e2e TestE2E_KfmThrowsKfm on the real kfm throw (800 -> 810 / custom 820 -> 821 -> 5100, life -78). The released defender is back in its own states; a round reset builds fresh runtimes. Shortcuts: target id/index, BindToTarget, the throw priority rules.
