---
date: 2026-11-18
status: accepted
---
# Opponent and hit-contact triggers, AnimTime, and their differences from Ikemen GO (item 042, last slice)

**Context:** kfm and kfm_zss read the opponent (`P2BodyDist X`, `P2StateNo`, `p2,time`) and what their own hits did (`MoveContact`, `MoveHit`, `HitDefAttr`, `GetHitVar(yvel)`). The reference is Ikemen GO, commit `54d4148a` (roadmap decision 038). Where this item says "MUGEN timing", Ikemen GO's source is followed; the differences that remain are the table below.
**Decision:**
- **Opponent data.** `Context.P2` is an `evaluator.Opponent`, a snapshot **by value** (match state, Time, Anim, Ctrl, state/move type, `GetHit`, contact counter, `HitDefAttr`, constants), set by `Tick` before each fighter acts. A pointer to the other fighter's `Context` was rejected: `Context` flows through the controller registry's indirect calls (decision 016), so anything it points to escapes, one heap allocation per fighter per tick. Measured: `TestTick_OpponentAndContactTriggers_WarmTickAddsNoAllocation`.
- **Order inside a tick** (Ikemen GO `CharList.action`, `char.go:14066-14090`: each fighter's `actionRun` in turn, then hit detection):

```
 Tick(n)
   P1 acts   sees P2 as tick n-1 left it (state, GetHit, contact counter)
      |        its controllers change its StateNo / Vel / MoveType at once
      v
   P2 acts   sees P1 AFTER its controllers and its movement (decision 029)
      |
      v
   hit detection, damage, applyHitReaction   (attacker: counter = 1, target; defender: GetHit, state 5000/5010/5020)
```
- **Redirections** `p2,` `enemy(n),` `enemyNear(n),` `target(id),` wrap the next primary (`p2,time > 1` is `(p2,time) > 1`; a `StateType`/`MoveType` comparison belongs to the redirection). They read the snapshot through a derived `Context`; a trigger the snapshot lacks (`var`, `numProj`, `random`...) is an `UnknownTriggerError`, never a value read from zeroed fields. `root,`, `parent,`, `helper,`, `playerID,`, `player,`, `partner,`, `stateOwner,` fail at parse time with "not supported yet" until helpers exist (item 029). A redirection to a fighter that does not exist reads 0, as Ikemen GO's undefined does.
- **Contact counter** (`Context.MoveContactTime`): set to 1 on the tick the hit lands, +1 after each unfrozen action, held through hit pause, reset when a state is entered unless its StateDef says `movehitpersist` (`char.go:14270`, `13125`, `12594`; `bytecode.go:5456`; default 0, `compiler.go:7072`).
- **HitDef values** are recorded on the defender's `GetHit` when the hit lands (`getHitFor`, `char.go:11680-11838`): `yvel`, `yaccel`, `fall`, `fall.yvel`, `fall.recover`, `fall.recovertime`, `ctrltime`, the HitDef `id`, and `animtype` chosen by `gethitAnimtype` (`char.go:8102`). The fall time counts in hit states and the flag clears on the first tick out of one (`char.go:12644-12673`). The defender's move type becomes H before the get-hit state is entered, as Ikemen GO does (`char.go:13123`).
- **`AnimTime`** is Ikemen GO's `curtime - totaltime` (`anim.go:617`): 0 on the last tick of the animation, negative before, `curtime + 1` for an animation ending in a held element. The engine's own elapsed counter is renamed `Context.AnimElapsed` (the WebAssembly `animTime` field, which the viewer reads as elapsed ticks, keeps its name and meaning).

**Differences from Ikemen GO (accepted, each with its reason):**

| Subject | Ikemen GO | Engine |
|---|---|---|
| Fighter 2's view of fighter 1's position | after `posUpdate`, inside `actionRun` (`char.go:12583`) | same since item 044: each fighter moves right after its states (decision 029) |
| `P2BodyDist Y` | the vertical gap of the two size boxes for an Ikemen GO character, `P2Dist Y` for a MUGEN one (`char.go:9366-9372`) | always `P2Dist Y`: the character's `ikemenversion` is not read. kfm and kfm_zss declare `mugenversion = 1.0`, so they are exact |
| Size box | stand, crouch, air and down boxes, plus `Width` controllers (`char.go:10484-10523`) | stand box (`size.ground.*`) for S, C and L, air box (`size.air.*`) for A; no `crouch.sizebox`, `down.sizebox` or `Width` |
| Who is P2 | the nearest enemy that is not in standby or over-KO (`char.go:15015-15075`) | the other fighter, always |
| Distances | `float32` | `float64` |
| `MoveGuarded`, `MoveReversed` | counters of guarded and reversed contacts | always 0: the engine has no guarding or `ReversalDef` yet |
| Contact counter on a hit | `-1`, turned into 1 by the same tick's `tick()` | 1 at once |
| `HitTime` of a ground hit | `air.hittime` when `yvel` is not 0, and +1 for a MUGEN character (`char.go:11792-11808`) | `ground.hittime` for a ground defender, unchanged since decision 018 |
| Defender lying down (L) | `down.velocity`, `down.hittime` (`char.go:11773-11787`) | takes the ground branch; Tick does not put a lying fighter into a get-hit state yet |
| Targets | `HitDef` targets, dropped when they leave the hit states (`char.go:10387`) | the opponent while it is in a hit state after one of this fighter's hits; same drop point (before the fighter acts); `NumTarget(id)` filters on the HitDef id |
| `P2Dist X/Y`, `P2BodyDist X` for a WinMUGEN character | truncated to an integer when `mugenversion`'s major is not 1 (`char.go:9324-9327`, `9338-9341`, `9359-9361`) | never truncated: `character` does not expose `mugenversion` yet; kfm and kfm_zss (1.0) are exact |
| Re-entering the current state | a ChangeState always runs `stateChange1`/`stateChange2` and `sb.init` (`char.go:6966-6994`); during a hit pause `stateChange2` waits for the pause to end (`char.go:6943-6944`, `12529`) | same since the review round: a ChangeState flags the context (`StateChanged`) and Tick enters the state on that flag, or marks the entry pending during a hit pause |
| `HitDefAttr` | the attr of the last HitDef; its state type is rewritten after a `hitonce` contact (`char.go:1098`) | the attr of the last `.cns` HitDef that applied; not rewritten; a HitDef in a `.zss` block is not acted on yet (decision 024) |
| `GetHitVar(isbound)` | bound by a target bind | always 0 (no binds) |
| `forcenofall`, `fall.damage`, `down.*`, guard values of a HitDef | recorded | not parsed |
| `GetHit.HitShakeTime` when the move type is not H | reset to 0 (`char.go:12665`) | not reset (a defender with no get-hit state keeps its shake) |
| `HitDef` attr with several state letters | the last one wins (`compiler.go:741`) | same; the `HitDefAttr` trigger takes all of them |
**Reason:** a by-value snapshot keeps the warm tick at its input-recognition baseline, and each difference above is either unobservable on the reference characters (kfm, kfm_zss) or needs a feature that has its own backlog item (physics order 044, now done by decision 029; helpers 029; guarding).
**Rejected alternatives:** *A `*Context` to the other fighter:* a heap allocation per fighter per tick (see above). *Reading 0 for a redirection the snapshot cannot answer:* a silent wrong value in an AI condition is worse than an unknown-trigger error the conformance run counts. *Keeping `Context.AnimTime` as the elapsed counter and computing the trigger elsewhere:* two meanings under one name.
