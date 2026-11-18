---
date: 2026-11-18
status: accepted
---
# Animation triggers: AnimElem, AnimElemTime, AnimElemNo, AnimExist, SelfAnimExist (item 042, second slice)

**Context:** kfm and kfm_zss time their moves on animation elements (`AnimElem = 3`, `AnimElemTime(2) >= 0`), and common1 tests `SelfAnimExist(anim + 3)`. The reference is Ikemen GO, commit `54d4148a` (roadmap decision 038): where item 042 says "MUGEN timing", Ikemen GO's is followed, and the differences with MUGEN's documented timing that the source shows are listed below.
**Decision:**
- **Data:** `Context.Anims map[int]air.Animation`, the program's existing `animIndex`, set once by `FighterProgram.NewRuntime` (built there for a program made without `NewFighterProgram`). A map header is copied with the `Context`, never the table; a lookup allocates nothing.
- **Playback:** the fighter's `(Anim, AnimTime)` is the whole state. Ikemen GO's `curelem`/`curelemtime` come from the shared `evaluator.ElementAt` (also what Tick's `currentFrame` now uses), and `curtime` from `curTime` (`anim.go:960-964`).
- **`AnimElemTime(e)`** = `curtime` minus the durations of the elements before `e` (`anim.go:621-634`); an element the animation lacks is Ikemen GO's undefined (`char.go:5573-5578`), read as 0 (`bytecode.go:1328-1333`), not an error. Wrong argument count is the only error.
- **`AnimElemNo(t)`** is a direct port of `anim.go:636-679`; before the animation began (`t < -curtime`, `char.go:5567`) it is undefined, read as 0.
- **`AnimElem = n [, op t]`** is parsed to one node: `AnimElemTime(n) op t`, `= 0` without a comma, skipped (false) when element `n` does not exist (`compiler.go:4297-4318`, `OC_jud8` `bytecode.go:2242`). `n` must be a positive integer and the first operator `=` (`compiler.go:1296-1311`); the operator after the comma is optional (default `=`), and `t` is an additive expression where Ikemen GO takes an integer.
- **`AnimExist` / `SelfAnimExist`:** Ikemen GO's `AnimExist` reads the animation table of the player the current animation came from (`c.animPN`, `char.go:5583-5595`), `SelfAnimExist` the character's own (`c.gi().animTable`, `char.go:5597-5602`). The engine has no custom state owner or `ChangeAnim2` yet, so both read `Context.Anims`; split them when one of those lands.
**Differences from MUGEN's documented timing (as Ikemen GO behaves):**

| Subject | MUGEN documentation | Ikemen GO (followed) |
|---|---|---|
| `AnimElemTime(e)` on the tick an animation wraps to its loop start | the loop element's time is 0 | `curtime` still equals the total time for that tick, so it reads the total (`anim.go:960-964`): kfm action 200 at AnimTime 12 reads `AnimElemTime(1) = 12`, then 1 at 13; `AnimElem = 1` never fires on a loop |
| `AnimElemTime` for an element the animation lacks | an error / false | undefined, 0 when compared, but `AnimElem = n` stays false |
| `AnimElem` operators | `=` and an optional second comparison | `!=` and `-` rejected at compile time, `n <= 0` rejected (`compiler.go:1296-1311`, `4300-4303`) |
| `AnimExist` | the current animation table | `animPN`'s table, `SelfAnimExist` the own table (`char.go:5583-5602`) |
**Reason:** one parse-time node and a handful of pure helpers over the existing per-program index keep the hot path allocation-free (`TestEval_AnimTriggersWarmPath_AllocateNothing`) and port the source exactly rather than approximating its timing.
**Rejected alternatives:** *Copying the animation table into `Context`*: a per-tick copy of a map's contents for no gain over its header. *Evaluating `AnimElem = n, op t` as a binary comparison with a comma operator*: the comma and the optional operator are not expression syntax, and the "n must be a positive integer" rule has no other home.
**Not done here (shortcut, noted in `evaluator/animation.go`):** an element held (`-1`) or of zero length *before* the last one (Ikemen GO's `animSeek` skips it, `anim.go:753-780`); no real animation has one.
