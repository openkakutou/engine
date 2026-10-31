---
status: todo
---
# Hit pause and hit-reaction states

## Description
A landed hit currently subtracts damage and builds combos (items 010/013) but neither fighter freezes for the hit's declared pause time, and the defender is not driven into a get-hit state for the declared stun time. Add hit pause for attacker and defender and hit-reaction state transitions (stand/crouch/air hit, slide time), driven by the HitDef values the attacker's state declares.

## Acceptance Criteria
- [ ] Both fighters freeze for the HitDef's pause time on a landed hit, with animation and state time held
- [ ] Defender enters the get-hit state matching its stance and stays in it for the declared stun/slide time
- [ ] A hit with missing pause/stun values falls back to a documented default instead of crashing
- [ ] Fixture-driven tests use a real character's HitDef values

## Notes
Decision 009/010 allocation discipline applies: no per-tick heap allocation on the hit path.
