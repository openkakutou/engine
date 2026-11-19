---
status: done
---
# HitDef in .zss Blocks

## Description
A `HitDef` written in a `.zss` state does nothing (decision 024: only `.cns` states report applied controllers). Since engine#040, the fidelity video shows the `.zss` Kung Fu Man attacking, but its hits never land and the opponent's life never moves. Make a `.zss` HitDef start a hit exactly as a `.cns` one does, through the same pipeline (hit detection, damage, get-hit state, contact counters, `HitDefAttr`).

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`), commit `54d4148a`.

## Acceptance Criteria
- [x] A `.zss` HitDef lands like the same `.cns` HitDef, in a test that compares the two
- [x] End-to-end: the `.zss` Kung Fu Man's light kick (state 230) damages the opponent by the HitDef's damage

## Notes
Found by the fidelity video after engine#040 (2026-11-19).
