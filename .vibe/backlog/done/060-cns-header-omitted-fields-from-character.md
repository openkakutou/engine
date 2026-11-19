---
status: todo
---
# CNS Header Omitted Fields from Character

## Description
For `.cns` states, engine#055 could not follow Ikemen GO fully: `cns.StateDef` loses whether `ctrl` and `anim` were written, and drops `velset`. So `.cns` entry still takes `ctrl` as written (false when omitted), treats `anim = 0` as omitted, and ignores the header's `velset` (a `shortcut:` in `enterState`). Use the written fields that character#079 provides, and remove the shortcut.

🔒 MUST: Ikemen GO behaviour (roadmap decision `038`), commit `54d4148a`, bytecode.go:5444-5512.

## Acceptance Criteria
- [ ] Entering a `.cns` state that omits `ctrl` keeps control; one that omits `anim` keeps the animation; a written `velset` applies
- [ ] The same for a `.cns` state sent over the WebAssembly JSON request

## Notes
Needs character#079 released. Found by item 055 (2026-11-19).
