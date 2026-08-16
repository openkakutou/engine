---
date: 2026-08-16
status: accepted
---
# `input.TickInput` models raw Left/Right, not facing-relative Back/Forward

**Context:** Building the per-tick command matcher (backlog item 008), raw input needed a shape. `.cmd` command strings are authored relative to the fighter's own facing (`F`/`B`), but a real input source (keyboard, gamepad) only knows absolute Left/Right.

**Decision:** `input.TickInput` carries raw `Up`/`Down`/`Left`/`Right`. `input.Step` takes the fighter's current `match.Facing` as a parameter and resolves Left/Right into forward/back internally, before matching against a parsed command's facing-relative steps.

**Reason:** A host/game-mode input reader shouldn't need to know about character facing just to report which keys are held; the command matcher already has to interpret `.cmd`'s F/B-relative syntax, so it's the natural owner of that translation. This also keeps `TickInput` a genuinely raw, facing-agnostic shape, simple to construct from any real input source.

**Rejected alternatives:** Requiring the caller to pre-resolve Left/Right into Forward/Back before calling `Step` — rejected: pushes the same translation onto every caller instead of centralizing it once, and couples every input source to `match.Facing` for no benefit.
