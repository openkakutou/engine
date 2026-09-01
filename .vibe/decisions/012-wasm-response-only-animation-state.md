---
date: 2026-09-02
status: accepted
---
# Animation number/timing exposed via a new response-only parallel array, not folded into FighterState

**Context:** Item 017 asks for a JS caller (starting with `mode-quick-versus`) to read each fighter's current animation number and elapsed frame timing from `newMatch`/`tick`/`resetRound`'s JSON response, without a Go toolchain — this data (`evaluator.Context.Anim`/`AnimTime`) exists only Go-side today, kept out of JSON per `.vibe/decisions/011`. The change must be additive-only: no existing response field changes, and no request-decoded type (`FighterState`-as-request included) gains a new field.

**Decision:**
1. Add a new response-only type, `FighterAnimState{AnimNo, AnimTime int}` (JSON `animNo`/`animTime`), and a parallel `Animations [2]FighterAnimState` array (JSON `animations`), indexed by `match.Side` the same way as the existing `Fighters`/`Programs`/`Starting` arrays. `match.FighterState` — reused for both the request-side `starting` payload and the response-side `state` payload — is left untouched, so the request surface gains nothing.
2. Named `animNo`, not `anim`, to match this codebase's existing "current X identifier" convention (`stateNo`), and to read unambiguously next to `stateNo` — a fighter's animation number and its state number are related but not the same thing (`cns.StateDef.Anim` can differ from the state number via `changeanim`) and must not look like the same field twice.
3. `Animations` is populated identically in all three response types (`newMatchResponse`, `tickResponse`, `resetRoundResponse`), straight from the session's post-tick `FighterRuntime.Context.AnimNo`/`.AnimTime` — the same values that feed the *next* tick's frame resolution. The engine's own internal frame-lag-by-one-tick behavior for hit detection (`.vibe/decisions/011`, point 2) is a separate, already-closed concern this item does not change or need to match.

**Reason:** A parallel array keeps `FighterState`'s request/response dual role stable (the narrowest cut satisfying the acceptance criteria), and `animNo` avoids a same-response naming collision risk with `stateNo` that would invite callers to assume the two always move together.

**Rejected alternatives:**
- **Add `Anim`/`AnimTime` fields directly onto `FighterState`**: rejected — `FighterState` is also the request-side `starting` shape; adding fields there would make them appear to round-trip on requests even though nothing reads them back, which is confusing and against the item's explicit additive-to-response-only constraint.
- **Name the field `anim`**: rejected — reads as ambiguous next to `stateNo` in the same response; `animNo` matches the existing abbreviation convention and is unambiguous.
