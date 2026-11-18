---
date: 2026-11-18
status: accepted
---
# ignorehitpause controllers run during hit pause; the .zss ignoreHitPause{} block is a known gap

**Context:** Backlog item 034. Decision 018 (point 7) made a fighter in hit pause (attacker freeze or defender shake) run no controllers and deferred `ignorehitpause` to this item. Compatibility target (MUST): MUGEN 1.0/1.1 and Ikemen GO. Sources: the MUGEN 1.1 cns documentation (a controller without `ignorehitpause = 1` "will not be checked during a hit pause") and Ikemen GO's `data/common1.cns.zss`, which wraps such logic in `ignoreHitPause{}` blocks. No Ikemen GO engine source is available locally, so the exact order of the freeze bookkeeping relative to these controllers is *unverified*.

**Decision:** This amends point 7 of 018 (018 itself is unchanged).
1. During a freeze, `tickFighter` runs states -3, -2, -1 and the current state through `statemachine.StepStateDuringHitPause`, which skips every controller whose `ignorehitpause` parameter is absent or evaluates false. The parameter is an expression evaluated by the evaluator; an invalid one is an error. Triggers of the flagged controllers are evaluated normally, and a skipped controller's triggers are not evaluated.
2. The frozen tick still advances no Time/AnimTime and runs no physics, and `HitPause` / `HitShakeTime` count down as in 018. A `ChangeState` by a flagged controller applies (`enterState`, as on an unfrozen tick). Sounds and unsupported-controller names are returned as on an unfrozen tick.
3. A frozen tick returns no `HasHitDef`: a HitDef controller does not start a hit while frozen, even if flagged (kept as in 018; *unverified* against Ikemen GO).
4. No allocation is added: a frozen tick running a flagged controller costs the same as an unfrozen tick running it (the applied-index list), measured with `testing.AllocsPerRun`.
5. The `.zss` block form `ignoreHitPause{}` is **not** implemented: `zssexec` is not wired into `Tick` yet, so there is nothing to attach it to. It is a gap to close when `zssexec` is wired in.

**Reason:** The parameter form is what `.cns` characters (MUGEN and Ikemen) use, and it fits the existing controller loop with one flag check. Wiring `zssexec` is a separate, larger item.

**Rejected alternatives:**
- Implementing the `ignoreHitPause{}` block in `zssexec` now: unreachable code until `zssexec` runs inside `Tick`.
- Changing `Step`/`StepState` signatures with a mode argument: touches every caller and test for one new behaviour; a separate exported function keeps them as they are.
- Running every controller during the freeze: breaks 018 and the MUGEN rule that unflagged controllers are not checked.
