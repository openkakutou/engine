---
status: done
depends_on: [047]
---
# .zss Grammar Used by Ikemen Common1

## Description
The `.zss` executor rejects 34 of the 62 blocks of Ikemen GO's `common1.cns.zss`. The failing forms are `else if`, `:=` assignment (e.g. `SysVar(1) := 0;`) and conditions spanning several lines. Every block must parse and run as in Ikemen GO.

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility, verified against Ikemen GO (roadmap decision `038`), grounded in its source (commit `54d4148a`).

## Acceptance Criteria
- [x] Every block of Ikemen GO's `common1.cns.zss` parses with no error
- [x] `else if`, `:=` and multi-line conditions each have a test matching Ikemen GO's behaviour, with the source cited
- [x] Malformed input still returns a clear error naming the block

## Notes
Split out of item 035 (Product Owner, 2026-11-18): the reference files are the `common1.cns` and `common1.cns.zss` shipped with Ikemen GO (MIT, roadmap decision `037`); acceptance characters are Ikemen GO's `kfm` and `kfm_zss`. Depends on the fixtures (047).
