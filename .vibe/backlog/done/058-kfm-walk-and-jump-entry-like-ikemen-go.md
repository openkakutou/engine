---
status: done
depends_on: [041]
---
# Kfm Walk and Jump Entry Like Ikemen GO

## Description
With intervals stubbed (found during item 042), the `.cns` Kung Fu Man in the end-to-end scenarios goes wrong in two ways:
- Holding forward makes it enter state 100 (run) instead of 20 (walk).
- Holding up never takes it out of state 0. `holdup` is active, but nothing changes the state to 40.

The cause is not diagnosed. Candidates are how the `FF` command matches a held direction, and the walk/jump entry that MUGEN/Ikemen GO hard-codes when a character's states do not do it.

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`), grounded in its source (commit `54d4148a`).

## Acceptance Criteria
- [x] The cause is found and cited from the Ikemen GO source
- [x] The `.cns` Kung Fu Man walks (state 20, X increasing) when forward is held, and jumps (state 40) when up is pressed, in the end-to-end scenarios
- [x] The `.zss` Kung Fu Man does the same, once its own blockers are gone

## Notes
Found by item 042 (2026-11-18), with a temporary interval stub that was reverted.
Done on 2026-11-19. The `.zss` Kung Fu Man walks, and both Kung Fu Man versions enter jump start then jump up. Still open, each in its own item: the `.cns` walk animation (item 061, OR between trigger groups) and landing (item 044).
