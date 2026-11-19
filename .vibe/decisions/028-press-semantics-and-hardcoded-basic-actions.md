---
date: 2026-11-19
status: accepted
---
# Commands match presses, and the engine enters walk, jump and crouch itself (item 058)

**Context:** In the end-to-end scenarios, holding forward put Kung Fu Man in run (100) instead of walk (20), and holding up left it in state 0. Ikemen GO (commit `54d4148a`) does neither:
- A step without `/` matches only when its key's buffer reads 1, the tick it is pressed (input.go:2621-2625); `/` matches while held. The engine matched a held key on every tick, so a held F completed kfm.cmd's `FF = F, F` on the second tick and kfm.cmd's state -1 changed to 100.
- common1 has no ChangeState into 20 or 40 for a player. Ikemen GO's actionPrepare changes to them itself for a fighter with control (char.go:12300-12345): jump 40 on up, crouch 10 on down, stand 12 out of a crouch, walk 20 on forward or back, and brake 0 out of 20 without control.

**Decision:**
- `input`: a step matches on a press unless it carries `/` (anywhere in its prefix, the whole step, as Ikemen GO propagates it, input.go:2480-2503). `State` keeps the previous tick's directions and buttons. A tick that matches no step keeps the partial sequence; only its time drops it (input.go:2566-2584). A step with no button followed by a non-held step not sharing its direction may match on the same tick (IsDirToButton, input.go:2031-2054, 2385-2407).
- Root: `basicActions` runs those entries before states -3/-2/-1, as a ChangeState with no anim or ctrl, entered after state -1 like one made there.

**Differences kept (shortcuts):** no AssertSpecial yet (item 050), so `nohardcodedkeys`, `nowalk`, `nojump`, `nocrouch`, `nostand`, `nobrake` are off, Ikemen GO's default. No guard distance: the guard entry (120) never fires and back always counts against forward for walking. No air jump (45), no round-over input lock. A character without the target state is left where it is (Ikemen GO enters an empty state). `~`, `$`, `>` still match as the plain step (backlog 025); a `~` step reads as a press.

**Rejected alternatives:** a `holdfwd`/`holdup` ChangeState added to the loaded states (not what Ikemen GO does; a character's own -1 would then run against invented states). Keeping held-key matching and special-casing `FF` (breaks every other command the same way, e.g. a held attack button repeating attacks).
