---
date: 2026-11-19
status: accepted
---
# A .zss HitDef is reported by zssexec, its locals resolved when it runs (item 062)

**Context:** Decision 024 left a known limit: a `HitDef` in a `.zss` block did nothing, because `tickFighter` found the HitDef through `statemachine.Result.Applied`, which only `.cns` states produce. The `.zss` Kung Fu Man's hits never landed. Ikemen GO (commit `54d4148a`) evaluates every HitDef parameter when the controller runs (`hitDef.Run`, `src/bytecode.go:8631-8650`, `runSub` `:8242`), so a `.zss` parameter that reads a local (`damage: $d`) takes the local's value at that moment; a local exists only while its block runs. The engine's `.cns` path reads HitDef parameters later, as raw text, in `combat.ParseHitDef` (literals only, decision 018).

**Decision:**
1. `zssexec.Result` gains `HasHitDef` and `HitDef cns.Controller`: the first HitDef that ran during the call, the same shape `tickFighter` used for `.cns`. The tick state is package-level, like `skippedCalls`, so the `Context` does not escape (no allocation, decision 009).
2. Each comma-separated piece of a parameter that contains `$` is evaluated with the running frame's locals and replaced by its number; the map is cloned only then. Any other value stays raw text. A HitDef with no local reuses the parsed map: nothing allocated. A piece that fails to evaluate is a tick error naming the parameter.
3. `FighterProgram.step` returns the HitDef for both formats (the `.cns` scan of `Applied` moved there); `tickFighter` has one HitDef path (hit, damage, get-hit entry, contact counters, `HitDefAttr`, pause times).
4. A frozen tick still starts no hit, an `ignoreHitPause` block's HitDef included (decision 021, point 3).

**Reason:** Substituting only the pieces that read locals is the smallest change that keeps Ikemen GO's "evaluated when it runs" for the values that would otherwise be lost, and keeps `.zss` and `.cns` identical for everything else.

**Known limits:** a parameter expression without a local (`damage: var(1)`) still falls back to its default, as in `.cns` (decision 018). The first HitDef of a tick is kept for both formats; in Ikemen GO each HitDef that runs resets and replaces the previous one (`bytecode.go:8637`), so the last one wins (*not changed here*).

**Rejected alternatives:**
- *Evaluating every parameter in zssexec*: `ParseHitDef` reads enum text (`attr`, `animtype`, `ground.type`) that is not an expression, and `.cns` would still differ.
- *Passing the locals to `tickFighter`*: they are gone once the block ends.
