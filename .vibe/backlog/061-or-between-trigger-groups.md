---
status: done
---
# OR Between Trigger Groups

## Description
The engine ANDs every trigger of a `.cns` controller. MUGEN and Ikemen GO evaluate every `triggerall` AND (`trigger1` OR `trigger2` OR …). As a result, controllers that rely on OR never fire, starting with Kung Fu Man's walk animation (`common1.cns:128-133`, the `.cns` walk end-to-end scenario). Evaluate the groups that character#080 provides, as Ikemen GO does, with the source cited (commit `54d4148a`).

🔒 MUST: MUGEN 1.0/1.1 and Ikemen GO compatibility (roadmap decision `038`).

## Acceptance Criteria
- [x] triggerall AND (OR of the numbered groups) is evaluated as Ikemen GO does, including its rules for gaps in the numbering, with the source cited
- [x] The `.cns` Kung Fu Man walk scenario plays animation 20
- [x] Controllers sent over the WebAssembly JSON request get the same semantics

## Notes
Needs character#080 released. Found by item 058 (2026-11-19).
