# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

## [2.25.0] - 2026-11-20

### Fixed
- A state change to a state the character does not have (a `ChangeState` in `.cns` or `.zss`) no longer stops the match with an error: as in Ikemen GO, the fighter enters an empty state numbered as requested, with its state type, move type and physics unchanged, and `TickResult.Warnings` (and the `warnings` field of the WebAssembly tick response) reports "changed to invalid state N (from state M)". A fighter found in a state the character lacks runs it as an empty state instead of erroring.

## [2.24.0] - 2026-11-20

### Changed
- Depends on `character` v0.18.0.

### Added
- `LoadFighterProgramFS(fsys, ch, defName, searchDirs...)` builds a fighter program from an `fs.FS` (names are slash-separated paths inside it), with the same search as `LoadFighterProgram`: the .def folder, then the search folders in order, the `.zss` fallback and the case-insensitive lookup. `LoadFighterProgram` shares the same search, so the programs it builds and its error messages do not change.

## [2.23.0] - 2026-11-19

### Changed
- The Go module path is now `github.com/openkakutou/engine/v2`, matching the v2 tags: consumers can `go get github.com/openkakutou/engine/v2@v2.23.0` instead of pinning a raw commit.
- Depends on `stage` v0.17.0 and `sff` v0.8.1, the versions `game` already uses.

## [2.22.2] - 2026-11-19

### Fixed
- Entering a `.cns` state no longer removes control or changes the animation when its header does not write `ctrl` or `anim`, and a header's `velset` now applies, as in Ikemen GO: Kung Fu Man keeps control while walking and can attack afterwards. A landed hit still removes the defender's control, as Ikemen GO's get-hit entry does.

## [2.22.1] - 2026-11-19

### Fixed
- A state change made by a state now runs the new state in the same tick, as in Ikemen GO, instead of one tick later: every transition is no longer a tick late, chained changes run one after the other (bounded at 2500 like Ikemen GO), and the landing is done by the character's own landing state.

## [2.22.0] - 2026-11-19

### Fixed
- A `.cns` controller now fires when all its `triggerall` lines hold and any one of its numbered `trigger1`, `trigger2`, ... groups holds, as in MUGEN and Ikemen GO, instead of requiring every trigger line at once. Evaluation stops as Ikemen GO does (later groups are not evaluated once one holds), and a controller without `trigger1` never fires. Kung Fu Man (`.cns`) now plays its walk animation, and its light punch (`x`) starts state 200 with its sound; this also applies to controllers sent to the WebAssembly build.

## [2.21.0] - 2026-11-19

### Fixed
- Fighters no longer walk through each other: when their size boxes (the character's `[Size]` ground or air width and height) and their hurt boxes overlap, they are pushed apart as in Ikemen GO, sharing the overlap by weight, the higher `PlayerPush` priority staying put, and a cornered fighter making the other take the whole push. Kung Fu Man walking into Kung Fu Man stops 32 apart and pushes the other back.
- The `PlayerPush` controller is supported (`value`, `priority`, `affectteam`); like in Ikemen GO it lasts only the tick it runs in.

## [2.20.0] - 2026-11-19

### Fixed
- A `HitDef` written in a `.zss` state now lands like the same `.cns` one: the opponent loses life, enters its get-hit state and both fighters pause. Its parameters may read the state's local variables (`damage: $d`). The `.zss` Kung Fu Man's attacks now hit.

## [2.19.0] - 2026-11-19

### Fixed
- A jumping fighter now lands: on touching the ground in an air state it enters the landing state (52), as in Ikemen GO, so a jump returns to standing.
- Fighters move in Ikemen GO's order within a tick: a velocity set by a state moves the fighter before hits are checked, and the second fighter sees where the first has just moved. Gravity is added after the move, so jump arcs match Ikemen GO (Kung Fu Man's neutral jump peaks at 84.4 and lands 40 ticks after leaving the ground).

## [2.18.0] - 2026-11-19

### Added
- `PlaySnd` accepts the `F`, `S` and `M` sound prefixes, glued (`S5, 0`) or spaced (`S $bank, $sound` in `.zss`), as in Ikemen GO. A sound event now says whether it plays from the common sound file (`F`) or the character's own sounds (`S` or no prefix), and carries its `channel` (-1 when absent) and `lowpriority`. The other `PlaySnd` parameters are accepted and ignored.

## [2.17.0] - 2026-11-19

### Fixed
- Fighters have control when the round starts, as in Ikemen GO, so a character whose standing state does not give control can still move.
- Entering a state whose header does not set control or an animation keeps the current control and lets the current animation run on; a `.cns` header that writes `ctrl = 0` still takes control away.
- State headers written as expressions are evaluated on entry, and their power gain, `facep2` turn and (in `.zss` states) starting velocity now apply.
- A `ChangeState` that sets `ctrl` or `anim` now applies them.
- A fighter with control walks while forward or back is held, jumps when up is held, crouches when down is held and stands up again, as in Ikemen GO, even when its own states never change into those states.
- Holding a direction or a button counts as one press: holding forward no longer triggers a double-tap dash, and holding an attack button no longer repeats the attack. A pause between two steps of a motion no longer cancels it while it is still within its time, and a button pressed together with the last direction of a motion completes it.

## [2.16.0] - 2026-11-19

### Added
- Interval comparisons in triggers and expressions: `Anim = [5051, 5059]`, `StateNo != [3000, 3050)`, with open or closed bounds, int or float, and after `AnimElem = n,`, as in Ikemen GO.
- The operators `**` (power), `&`, `|`, `^`, `~` (bitwise) and `^^` (logical exclusive or), with Ikemen GO's precedence.
- `Cond(condition, a, b)`, which evaluates only the branch it picks.
- The conformance command takes `-common <folder>` to find a character's common state file.

### Fixed
- `=` and `!=` now bind more loosely than `<`, `>`, `<=` and `>=`, as in Ikemen GO.
- Dividing two integers gives a truncated integer, and `%` works on integers, as in Ikemen GO.
- Kung Fu Man (`.cns` and `.zss`) now stands idle through the end-to-end scenarios instead of stopping on an interval or `Cond`.

## [2.15.0] - 2026-11-19

### Added
- Characters can read their own constants with `Const(name)` (`data.*`, `size.*`, `velocity.*`, `movement.*` and the `[Constants]` section, read from the `.def` "cns" file over Ikemen GO's defaults) and use `Const240p`/`Const480p`/`Const720p`/`Const1080p`.
- Float variables: `FVar(n)`, `SysFVar(n)`, and the `fvar`/`sysfvar`/`fv`/`sysfv` forms of `VarSet` and `:=`.
- The animation triggers `AnimElem` (`AnimElem = n` and `AnimElem = n, op t`), `AnimElemTime(n)`, `AnimElemNo(t)`, `AnimExist(n)` and `SelfAnimExist(n)`, timed as Ikemen GO does.
- The triggers `AILevel`, `AILevelF`, `PalNo`, `NumHelper` and `NumExplod`.
- Characters can read their opponent: `P2Dist`, `P2BodyDist` (from the size boxes), `P2StateNo`, `P2MoveType`, `P2StateType`, `P2Life`, `NumEnemy`, `NumTarget`, and the redirections `p2,`, `enemy,`, `enemyNear,` and `target,` (`root,`, `parent,` and the helper ones fail with "not supported yet").
- Hit contacts: `MoveContact`, `MoveHit`, `MoveGuarded`, `MoveReversed`, `HitDefAttr = SCA, NA`, `HitFall` and `CanRecover`, and the rest of `GetHitVar` (`yvel`, `yaccel`, `fall`, `fall.yvel`, `fall.recover`, `fall.recovertime`, `ctrltime`, `isbound`), recorded from the HitDef when a hit lands.

### Changed
- `AnimTime` now reads as in Ikemen GO: the animation's time so far minus its total time, so 0 on its last tick and negative before (a held last element reads one more than the ticks played). The context's elapsed-ticks counter is now `Context.AnimElapsed`; the WebAssembly `animTime` field is unchanged. Code that read `Context.AnimTime` as elapsed ticks (for example to pick the frame to draw) must read `Context.AnimElapsed`.

### Fixed
- A ChangeState into the state the fighter is already in now re-enters it as Ikemen GO does: its animation restarts and the contact counter clears, so a move that cancels into itself can hit again.
- Entering a state clears the last HitDef unless the state says `hitdefpersist`, so `HitDefAttr` no longer matches a HitDef from an earlier move.
- A state change made during a hit pause applies its entry (animation, control, contact reset) once the pause ends, as in Ikemen GO.

## [2.14.0] - 2026-11-18

### Added
- Games can load every state file of a character the way Ikemen GO does, `.cns` and `.zss`, including its common states (`stcommon`), which are searched in the character folder and then in folders the game provides; files found nowhere are reported.

## [2.13.0] - 2026-11-18

### Added
- Characters written in `.zss`, or mixing `.cns` and `.zss` states, now play in a match: a single state table as in Ikemen GO, `ignoreHitPause` blocks running during a hit pause, and calls that do nothing reported each tick (also in the WebAssembly response). The conformance harness now runs `.zss` characters too.

## [2.12.0] - 2026-11-18

### Added
- `.zss` state scripts now accept Ikemen GO's full grammar used by its common states: `else if`, statements over several lines, local variables, functions with arguments and return values, `persistent`, and the `:=` assignment, including inside conditions. Ikemen GO's `common1.cns.zss` now reads with no error.

## [2.11.1] - 2026-11-18

### Fixed
- Characters that set their variables with the `var(N) = value` form, or the `sysvar`/`sysv` forms, no longer stop with an error; float variable forms now report a clear "not supported yet" error.

## [2.11.0] - 2026-11-18

### Added

- While frozen by a landed hit, a fighter now still runs the state controllers its character marks to ignore the hit pause (hit sparks, sounds and similar effects), as in MUGEN and Ikemen GO; every other controller stays paused.

## [2.10.0] - 2026-11-18

### Added

- Characters can now move and change state with the controllers every real character relies on: VelSet, VelAdd, VelMul, PosSet, PosAdd, ChangeAnim (including its starting element), CtrlSet, StateTypeSet, Turn, LifeAdd, LifeSet, Gravity and Null, in both `.cns` states and `.zss` scripts, following Ikemen GO's own rules.
- Each tick now reports, per fighter, the controller types a character used that the engine does not support yet, instead of silently ignoring them.

### Changed

- Gravity now applies automatically only in states whose physics is "air", as in Ikemen GO; other states fall only through their own Gravity or velocity controllers. A fighter that is hit now reacts according to its current state type (standing, crouching or airborne), including a type changed during the state.

## [2.9.0] - 2026-11-18

### Added

- Characters' conditions can now read the fighter's own life, power, facing, position, velocity, state type, move type, previous state, round number and whether it is alive or a helper, draw `Random` numbers, and use the usual math functions; `Random` follows Ikemen GO's generator from a seed kept in the match, so the same seed and inputs replay the same match.

## [2.8.0] - 2026-11-18

### Added

- Characters' special states (-3, -2 and -1, where command-driven moves are written) now run every tick, in Ikemen GO's order, ahead of the current state: a command pressed in a character's own files can now move the fighter into a special move, which starts in that same tick.

## [2.7.0] - 2026-11-17

### Added

- A conformance tool runs real characters against themselves with simulated inputs and reports which state controllers and triggers the engine does not support yet, how often each is reached, and which characters fail to load or crash, so the next combat features can be chosen from real data.

## [2.6.1] - 2026-11-17

### Fixed

- A hit whose damage is written with two values (hit damage and guard damage, the usual form in real characters such as Kung Fu Man) now deals its hit damage instead of none; the guard damage is kept for blocking.

## [2.6.0] - 2026-11-17

### Added

- A landed hit now freezes both fighters for the hit's declared pause, and the fighter who was hit enters its standing, crouching or airborne "get hit" state, so a character's own hit-reaction states play out (shake, knock-back, recovery) using the hit's stun and slide times; missing values use MUGEN's documented defaults.

## [2.5.1] - 2026-10-06

### Fixed

- A round now ends when its time runs out: the round timer counts down every tick, and reaching zero ends the round in favor of the fighter with more health (a draw if equal), instead of the match only ever ending by KO. A round started with a timer of 0 stays untimed and never ends on time.

## [2.5.0] - 2026-09-26

### Added

- A character's `PlaySnd` state controller now actually triggers: each fighter's triggered `(group, sample)` sound events are reported on every WebAssembly `tick` call, alongside the existing state and animation data. `engine` only reports which sound was triggered — decoding and playing the actual audio stays each consuming app's own job.

## [2.4.1] - 2026-09-25

### Fixed

- Adding a new non-state-changing state-controller type to the simulation loop (e.g. a future `PlaySnd`) no longer requires editing the core controller-dispatch code — it is registered instead, with no change in behavior for existing `ChangeState`/`VarSet`/`PowerAdd` controllers.

## [2.4.0] - 2026-09-23

### Added

- Fighters now have a power/meter (super gauge) value: it starts at 0 at the beginning of a match and at every round reset, is raised or spent by a `PowerAdd` state controller (clamped between 0 and 3000), and is reported alongside health in every match state update, including through the WebAssembly `newMatch`/`tick`/`resetRound` calls.

## [2.3.0] - 2026-09-22

### Added

- Tagged releases now publish a ready-to-use WebAssembly build: pushing a version tag builds and tests the library, then attaches `engine.wasm` and its matching `wasm_exec.js` to that release, so consuming apps can download a real build instead of compiling it themselves.

## [2.2.0] - 2026-09-22

### Added

- Faster per-tick simulation: a fighter's current animation and a `.zss` script's state/function blocks are now looked up instantly instead of being rescanned every time, with no change in behavior.

## [2.1.1] - 2026-09-21

### Fixed

- Removed leftover dead code flagged by a code review pass: an unused test helper, unused intermediate variables in a command-matching test, and unused instructions in the `wasm-lang-spike` benchmark's bytecode interpreter. No behavior change.

## [2.1.0] - 2026-09-02

### Added

- The WebAssembly module now reports each fighter's current animation number and how long it's been playing on every match update (start, tick, and round reset), so a game consuming it can show the right sprite frame without recreating the fighting logic itself.

## [2.0.1] - 2026-09-01

### Fixed

- Cleaned up several internal variable and function names flagged as confusing by a code review pass; no behavior change.

## [2.0.0] - 2026-09-01

### Fixed

- **Breaking:** Reduced how much of a MUGEN hit definition's internal data the damage-resolution code depends on, so it only reads the declared damage amount instead of the whole hit definition. No change to the WebAssembly-facing API, but any code calling the Go `combat.ApplyHits` function directly needs to pass a `DamageParams` value instead of the full controller.

## [1.0.0] - 2026-09-01

### Added

- A match session can now be explicitly released once a game is done with it (a new `closeMatch` function), instead of staying resident in memory for the life of the WebAssembly module.

### Fixed

- **Breaking:** Reduced complexity in the trigger-expression parser/lexer and simplified the per-tick simulation function's parameter list by grouping the stage boundaries, gravity, and combo-window constants into a single configuration value. No change to the WebAssembly-facing API, but any code calling the Go `Tick` function directly needs to pass the new `TickConfig` value instead of four separate parameters.

### Security

- Updated the underlying Go runtime to pick up 28 upstream standard-library security fixes, and added a safety limit that stops a maliciously crafted state script from crashing the simulation through unbounded recursion.

## [0.8.0] - 2026-08-30

### Added

- The engine can now run a full match, tick by tick, tying every piece together: input, combat-logic execution, physics, and hit/damage resolution for both fighters, followed by deciding whether the round just ended (a knockout, a double knockout, or the round timer running out with the healthier fighter winning) and tracking who has won how many rounds across a best-of-N match. A finished round can be reset into a fresh one with both fighters restored to their starting position and health. This is also the engine's first WebAssembly release: a downloadable module a game can load in the browser to start a match, advance it one tick at a time, and reset between rounds, without needing a Go toolchain of its own.

## [0.7.1] - 2026-08-30

### Fixed

- Fixed inconsistent code formatting in the `wasm-lang-spike` benchmark spike; no behavior change.

## [0.7.0] - 2026-08-28

### Added

- The engine can now turn a detected hit into an actual match effect: damage is subtracted from the defender's health (never below zero, even on an overkill hit), and a combo counter tracks consecutive hits, resetting once too much time passes between them. A hit whose declared damage amount is missing or unreadable still lands using a safe default instead of crashing the match.

## [0.6.0] - 2026-08-28

### Added

- The engine can now detect when two fighters' collision boxes actually overlap: an attacker's hit boxes against a defender's vulnerable boxes, using each fighter's real current animation frame, correctly positioned and mirrored for the fighter's facing direction. A frame with no collision boxes at all (like most idle frames) simply produces no hits, without any special handling needed.

## [0.5.0] - 2026-08-20

### Added

- The engine can now run a character's Ikemen GO `.zss` state scripts (the alternative to classic combat-logic files), not just classic ones: conditions are checked, variables can be updated, the fighter's state can change, and a helper script can be called from another one — producing the same real transition/variable results a classic combat-logic file would for the same behavior. A script using something not supported yet reports a clear error instead of silently doing nothing or crashing.

## [0.4.0] - 2026-08-18

### Added

- The engine can now simulate a fighter's physics tick by tick: gravity pulls it down while airborne, it lands cleanly back on the ground, and it can never be pushed outside the current stage's boundaries no matter how fast it was moving.
- The engine can now read a player's raw per-tick input (directions and button presses) and recognize a character's special-move motions (e.g. quarter-circle-forward + punch) as they're entered, within the same timing window the original command file declares — including correctly rejecting a close-but-incomplete motion, and correctly forgetting a motion started too long ago to still count. A recognized motion becomes available to the same combat-logic conditions that already check for it.

## [0.3.0] - 2026-08-09

### Added

- The engine can now drive a fighter through its character's state definitions — checking each state's conditions in order, applying variable updates and state changes for whichever conditions are met, and reporting a clear error if a state change targets a state that doesn't exist.

### Fixed

- Removed a leftover local dependency override that would have prevented anyone outside this workspace from building against this module; it now resolves the `character` dependency the standard way, from its published releases.

## [0.2.0] - 2026-08-09

### Added

- The engine can now parse and evaluate MUGEN CNS trigger/expression syntax — comparisons, boolean and arithmetic operators, and built-in triggers such as `Time`, `Ctrl`, `Anim`, `Command`, `var()`, `sysvar()`, and `IfElse()` — against a fighter's live state, returning a clear error instead of a wrong or default result for malformed or unsupported expressions.

## [0.1.0] - 2026-08-09

### Added

- The engine now models the live state of a match while two characters fight — each fighter's position, facing, movement, and current state, plus the round number and round timer — as the foundation later combat-simulation features build on.

[Unreleased]: https://github.com/openkakutou/engine/compare/v2.25.0...HEAD
[2.25.0]: https://github.com/openkakutou/engine/compare/v2.24.0...v2.25.0
[2.24.0]: https://github.com/openkakutou/engine/compare/v2.23.0...v2.24.0
[2.23.0]: https://github.com/openkakutou/engine/compare/v2.22.2...v2.23.0
[2.22.2]: https://github.com/openkakutou/engine/compare/v2.22.1...v2.22.2
[2.22.1]: https://github.com/openkakutou/engine/compare/v2.22.0...v2.22.1
[2.22.0]: https://github.com/openkakutou/engine/compare/v2.21.0...v2.22.0
[2.21.0]: https://github.com/openkakutou/engine/compare/v2.20.0...v2.21.0
[2.20.0]: https://github.com/openkakutou/engine/compare/v2.19.0...v2.20.0
[2.19.0]: https://github.com/openkakutou/engine/compare/v2.18.0...v2.19.0
[2.18.0]: https://github.com/openkakutou/engine/compare/v2.17.0...v2.18.0
[2.17.0]: https://github.com/openkakutou/engine/compare/v2.16.0...v2.17.0
[2.16.0]: https://github.com/openkakutou/engine/compare/v2.15.0...v2.16.0
[2.15.0]: https://github.com/openkakutou/engine/compare/v2.14.0...v2.15.0
[2.14.0]: https://github.com/openkakutou/engine/compare/v2.13.0...v2.14.0
[2.13.0]: https://github.com/openkakutou/engine/compare/v2.12.0...v2.13.0
[2.12.0]: https://github.com/openkakutou/engine/compare/v2.11.1...v2.12.0
[2.11.1]: https://github.com/openkakutou/engine/compare/v2.11.0...v2.11.1
[2.11.0]: https://github.com/openkakutou/engine/compare/v2.10.0...v2.11.0
[2.10.0]: https://github.com/openkakutou/engine/compare/v2.9.0...v2.10.0
[2.9.0]: https://github.com/openkakutou/engine/compare/v2.8.0...v2.9.0
[2.8.0]: https://github.com/openkakutou/engine/compare/v2.7.0...v2.8.0
[2.7.0]: https://github.com/openkakutou/engine/compare/v2.6.1...v2.7.0
[2.6.1]: https://github.com/openkakutou/engine/compare/v2.6.0...v2.6.1
[2.6.0]: https://github.com/openkakutou/engine/compare/v2.5.1...v2.6.0
[2.5.1]: https://github.com/openkakutou/engine/compare/v2.5.0...v2.5.1
[2.5.0]: https://github.com/openkakutou/engine/compare/v2.4.1...v2.5.0
[2.4.1]: https://github.com/openkakutou/engine/compare/v2.4.0...v2.4.1
[2.4.0]: https://github.com/openkakutou/engine/compare/v2.3.0...v2.4.0
[2.3.0]: https://github.com/openkakutou/engine/compare/v2.2.0...v2.3.0
[2.2.0]: https://github.com/openkakutou/engine/compare/v2.1.1...v2.2.0
[2.1.1]: https://github.com/openkakutou/engine/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/openkakutou/engine/compare/v2.0.1...v2.1.0
[2.0.1]: https://github.com/openkakutou/engine/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/openkakutou/engine/compare/v1.0.0...v2.0.0
[1.0.0]: https://github.com/openkakutou/engine/compare/v0.8.0...v1.0.0
[0.8.0]: https://github.com/openkakutou/engine/compare/v0.7.1...v0.8.0
[0.7.1]: https://github.com/openkakutou/engine/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/openkakutou/engine/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/openkakutou/engine/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/openkakutou/engine/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/openkakutou/engine/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/openkakutou/engine/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/openkakutou/engine/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/openkakutou/engine/releases/tag/v0.1.0
