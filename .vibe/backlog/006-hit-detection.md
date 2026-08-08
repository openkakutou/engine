---
status: todo
depends_on: [003, 004]
---
# Hit Detection

## Description
Resolve Clsn (collision box) overlap between two fighters each tick: hit boxes (Clsn1, attack) against hurt boxes (Clsn2, vulnerable), sourced from the currently active animation frame's collision data already exposed by `character/air` (`Frame`/`ClsnBox`, pre-resolved per that repo's decision record). Detection must work regardless of whether a fighter's current state is being driven by CNS state-machine execution (item 003) or `.zss` script execution (item 004), since both produce the same underlying animation/frame progression. A detected hit yields a hit event (attacker, defender, which boxes overlapped) that item 007 consumes — this item does not apply damage itself, only detects and reports collisions. "Done" means two fighters animated through real character data collide (and don't collide) exactly when their real Clsn boxes say they should, verified against hand-computed box geometry, not just a passing/failing bool.

## Acceptance Criteria
- [ ] Overlapping Clsn1 (attacker) and Clsn2 (defender) boxes for the current animation frame of each fighter are detected as a hit event
- [ ] Non-overlapping boxes correctly produce no hit event
- [ ] Box positions account for fighter position and facing (a box is mirrored correctly when a fighter faces left vs. right)
- [ ] A fighter whose current frame has no Clsn boxes at all (a valid case, e.g. an idle frame) produces no hit event without erroring
- [ ] Unit tests include a real character's animation frame data (via `character/air`) exercised through both a colliding and non-colliding relative position

## Notes
None.
