---
date: 2026-08-28
status: accepted
---
# Hit detection: local-to-world Clsn box transform, and zero-allocation zero-hit ticks

**Context:** Backlog item 006 resolves `character/air`'s `Frame.Clsn1`/`Clsn2` boxes against two fighters' live `match.Position`/`Facing` to detect hits. `air.ClsnBox` stores `Left/Top/Right/Bottom` in `.air`'s own local coordinate space (confirmed against a real fixture: `Clsn2[0] = -6,-92,6,0` on a standing character — Top is the more-negative value, near the head), which increases **downward** like screen/sprite-pixel space. `match.Position.Y` follows the opposite, "upward is positive" convention (`physics` package doc). This item also runs unconditionally every simulation tick for the whole match, inside `mode-quick-versus`'s real-time frame budget, on a `GOOS=js GOARCH=wasm` (single-threaded) build where GC mark-assist work interleaves synchronously with the simulation loop.

**Decision:**
1. Converting a local `ClsnBox` to a world-space AABB flips the Y axis (`worldY = fighterPosition.Y - localY`) in addition to the existing X-axis facing-mirror, computed inline as local `float64` values per box pair — never into an intermediate heap-allocated "transformed box" type.
2. `Detect` returns a `nil`-by-default `[]HitEvent`, grown only by `append` on an actual overlap — never pre-sized (`make([]HitEvent, 0, N)`) — so the common zero-hit tick allocates nothing.
3. The nested Clsn1×Clsn2 comparison (bounded, small counts per MUGEN convention) stays plain arithmetic with no interface dispatch or spatial-partitioning structure.

**Reason:** The Y-flip is necessary correctness, not a style choice — without it, a standing character's overlap boxes would compute upside-down relative to `Position`. The allocation discipline is necessary for a per-tick hot path on a single-threaded WASM target, where a well-intentioned pre-sized slice would allocate on literally every tick of a match regardless of whether anything hit, directly costing frame budget rather than being absorbed by a background/parallel collector.

**Rejected alternatives:**
- *Storing world-space boxes on `HitEvent` instead of the original local `air.ClsnBox` values* — rejected: the local values are what a consumer (item 007, or a future debug overlay) can meaningfully compare back against the source `.air` data; recomputing world space from `Position`+`Facing`+the local box is cheap and lossless, so nothing is gained by carrying the transformed copy forward.
- *A spatial partitioning/broad-phase structure for the box comparison* — rejected as premature: real MUGEN characters define a handful of Clsn boxes per frame (bounded, small n), so an O(n×m) nested loop is already well inside budget; partitioning solves a scale problem this workload doesn't have.
