---
date: 2026-08-09
status: accepted
---
# Step resets the state timer on a state change but does not auto-increment it per tick

**Context:** Implementing backlog item 003 (state machine execution). `evaluator.Context.Time` (item 002) is "ticks elapsed since entering the current StateNo". Item 003's acceptance criteria require that a state-change controller "resets per-state execution position correctly" — i.e. `Time` must read 0 immediately after a state change, matching MUGEN's behavior of a freshly-entered state always starting at `Time = 0`.

**Decision:** `statemachine.Step` resets `Time` to 0 in its result exactly when a `ChangeState`-equivalent controller applies during that call, and otherwise leaves `Time` exactly as it was in the `Context` it was given — it does not increment `Time` by 1 on every call as a general per-tick counter.

**Reason:** Advancing `Time` by one tick regardless of a state change is simulation-loop bookkeeping (when does a tick happen, how many state machines run per tick, ordering against physics/round flow), which belongs to round/match flow (a later, not-yet-built item) rather than to "interpret this state's controllers once". Making `Step` own only the reset keeps its contract narrow and directly testable (`Time` is 0 right after a change, unrelated to how the caller drives ticks) without speculatively designing the future per-tick loop now.

**Rejected alternatives:**
- **`Step` always increments `Time` by 1 unless a state change occurs:** bakes in a per-tick loop assumption (exactly one `Step` call = exactly one simulation tick) that this item's acceptance criteria do not require and that round/match flow (a future item) may need to shape differently (e.g. freezing `Time` during hitstop).
