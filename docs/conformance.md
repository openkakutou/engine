# Real-character conformance harness

The harness runs real MUGEN / Ikemen GO characters through the engine and
reports which controller types and triggers they use that the engine does not
support yet. It turns "the engine is early-stage" into numbers that can be
tracked over time and picks the next controllers to build (backlog item 030).

## Running it

```
go run ./cmd/conformance [-ticks N] [-json] [-common DIR] <path>...
```

Each path is a character `.def` file or a folder searched recursively for
`.def` files. A `.def` that is not a character (no `[Files]` section, e.g. a
storyboard) is reported as `skipped`. `-ticks` defaults to 3600 per character;
`-json` writes JSON instead of text; `-common` adds a folder searched for state files after the `.def` folder (the game's `data/`, where `stcommon = common1.cns` lives). A path that cannot be read stops the
command with exit code 1; any other per-character failure is recorded in the
report and the run goes on.

The output is deterministic: the same input gives byte-identical output.
Characters appear in argument order (folders in lexical order, paths relative
to the argument), counts are sorted by count descending then name. It contains
only paths, controller type and trigger names, numbers and error messages;
an error message from character parsing may quote a line from the character's
files.

## Method

1. **State table.** `character.Load` only parses the `.def` `cns` file into
   `Character.StateDefs`, so the harness calls `engine.LoadFighterProgram`, which also
   parses the other state files and merges them the way Ikemen GO does: `st`, `stN`, the `.cmd` file, then
   `stcommon`; the first definition of a state number wins (a character's own
   states override its common file's). `Character.StateDefs` is a last
   fallback. Files are searched in the `.def` folder, then in the extra directories the
   caller gives (the harness gives the `-common` folder, none by default). Missing files (typically
   `stcommon = common1.cns`, which lives in the game's `data/`) are skipped
   and returned to the caller. `.zss` entries are merged into the same
   table in the same order, first definition winning whatever the format
   (`engine.NewFighterProgramFromFiles`, decision `024`); a `.zss` that does not
   parse is a `load-error`.
2. **Mirror match.** The character fights itself: both sides run the same
   program, start in state 0 facing each other 140 units apart (x = -70 and
   +70) with 1000 health, in a stage bounded at x = -500..500, round timer 99 s
   (5940 ticks), gravity 0.44. When a round ends (KO or timeout) the round is
   reset as in `integration_test.go`, so the requested tick count is reached.
3. **Input script.** Each side follows a built-in deterministic script (no CPU,
   no randomness): neutral, forward, back, down, up, each button
   `a b c x y z s`, down + button, then the motions quarter-circle forward + `a`,
   dragon punch + `b` and back-charge forward + `c`. Directions are relative to
   the facing. The second side runs the same script offset by half its length.
4. **Neutralization.** The engine stops on an unknown trigger, so the run uses
   a copy of the state table in which a controller whose triggers mention a
   trigger rejected by `evaluator.IsKnownTrigger` -- or, when its type is
   supported, whose evaluated parameters (`value`, `v`) do -- gets its triggers
   replaced by `0` (never true). A controller whose trigger fails to parse is
   neutralized too and counted under "controllers with an unparsable trigger". The engine is never modified.
5. **Exposure.** Per fighter per tick, from the state the fighter is in when
   the tick starts: +1 for each unsupported controller type present in that
   state's controllers (once per type per state per tick), +1 for each unknown
   trigger name used by that state's controllers (once per name per state per
   tick; a function call is written `name()`). The report also gives distinct
   states reached / total states, and supported / total controllers in the
   reached states. The aggregate sums exposure per type or trigger over all
   characters and counts the characters exposed to it.
6. **Failures.** A `Tick` error stops that character's run (status `tick-error`,
   the message and the number of ticks completed are recorded) and the next
   character runs. A character whose states exist only in `.zss` runs like any
   other; every character reports its count of `.zss` `Statedef` blocks.

## Limitations

- Exposure counts **state presence**, not trigger evaluation: a controller
  counts every tick its state is current, whether or not its other triggers
  would have let it fire.
- `.zss` states run, but they are neither neutralized nor measured: a `.zss`
  element the engine rejects (an unknown trigger, a block that does not parse)
  stops the character's run as a `tick-error` naming the block, so the
  exposure behind it is not counted. They count as states and as reached.
- Neutralized controllers do not run, so they change the run: a neutralized
  `ChangeState` can keep a fighter out of states the real engine would reach,
  hiding the exposure behind them. Raise coverage, then re-run.
- Only the `value` and `v` parameters of supported types are inspected; a
  parameter that does not parse is ignored (PlaySnd's `group,sample` pair is split and each half inspected).
- Negative states split across several files are not merged (Ikemen GO merges
  them for characters flagged with an Ikemen version); the first file wins.
- Controllers with an unparsable trigger are counted over the whole state table, not only the
  reached states. `common1.cns` from the game's `data/` is not searched.

## Results

Run on 2026-11-17, `-ticks 3600`, over a local Ikemen GO install's character collection: 717 `.def` files, all characters that load in Ikemen GO. No character data is committed, only these figures.

| Outcome | `.def` files |
|---|---|
| ran the full 3600 ticks | 442 |
| stopped by an engine error | 28 |
| could not be loaded | 158 |
| skipped (not a character `.def`) | 89 |

### Named characters

The roadmap acceptance characters (decision `036`): Ikemen GO's bundled `kfm` was run; `kfm_zss` is not in this collection, so it was not run. All characters below declare a MUGEN version in their `.def` and run in Ikemen GO.

| Character | Status | States reached | Supported controllers (reached states) | Controllers with an unparsable trigger | Most exposed unsupported |
|---|---|---|---|---|---|
| `kfm` (Ikemen GO bundle, `mugenversion = 1.0`) | load error: no state 0 (its `stcommon` lives in the engine's `data/` folder, not searched by the harness) | — | — | 0 | — |
| Kung Fu Man (community edit, `1.0`) | ran | 1 / 156 | 1 / 5 | 313 | `changeanim`, `ctrlset`, `destroyself` |
| Link (`04.14.2001`) | ran | 1 / 234 | 1 / 6 | 193 | `changeanim`, `removeexplod`, `stopsnd` |
| Rayman | ran | 2 / 121 | 2 / 5 | 314 | `changeanim`, `velset` |
| Bridget (`04,14,2001`) | ran | 2 / 324 | 3 / 9 | 879 | `changeanim`, `destroyself`, `parentvarset` |
| Black Widow (`1.0`) | ran | 3 / 225 | 6 / 14 | 287 | `changeanim`, `explod`, `projectile` |
| Arale (`05,18,2010`) | engine crash at tick 102 (command matcher) | 1 / 123 | 1 / 4 | 125 | `changeanim`, `velset`, trigger `alive` |

### Whole collection

**Main finding: almost no character leaves its first state.** Of the 442 that ran, only 4 reached more than one state (447 states reached out of 105,635 defined). The engine runs only the fighter's current state each tick (`tick.go:325`, `statemachine.Step(ctx, prog.States)`), never the special states -3, -2 and -1 where characters put their command-driven state changes, so no input moves a fighter. Exposure figures below therefore mostly describe state 0: a floor, not a ranking of the whole engine.

Most exposed unsupported controller types (exposure, characters): `changeanim` (3,176,706; 470), `velset` (1,866,272; 280), `destroyself` (915,066; 146), `removeexplod` (367,202; 52), `posset` (273,632; 51), `explod` (165,620; 33), `assertspecial` (151,206; 24), `ctrlset` (136,800; 19).

Most exposed unknown triggers: `alive` (2,967,330; 436), `ishelper` (907,864; 144), `prevstateno` (468,042; 72), `animelem` (331,220; 56), `roundstate` (316,818; 53), `random` (172,838; 33), `fvar()` (172,804; 26), `palno` (158,400; 22), `numhelper()` (151,212; 27), `numenemy` (115,208; 20), `numexplod()` (108,016; 23). `s4000`-style names (11 characters) are `PlaySnd` common-sound groups read as triggers.

Controllers with an unparsable trigger: 349,244 across 470 characters. The report holds no expression text; a separate sampling of five characters' triggers through the evaluator showed interval syntax (`[a,b]`) as the largest identifiable cause, not the majority.

Engine errors that stopped a run: `VarSet` written without its `v` parameter, i.e. the `var(N) = value` form (21 characters); an index-out-of-range panic in the command matcher (7), reproduced on Arale by two commands sharing a name with different lengths (`TestRun_EnginePanic_IsRecordedAsTickErrorAndNextCharacterStillRuns`); the other 6 share the same panic message, cause not individually confirmed.

Load failures (158): no state 0 (81), `.air` frame line with fewer than 5 fields (26), `[Statedef N ...]` header followed by extra text (25), missing animation file (15, mostly backup folders), `[State]` block before any `[Statedef]` (4), malformed `.air` action header or Clsn line (4), missing combat logic file (1), missing sound file (1), truncated sprite data (1).

**Update, special states (item 037).** The engine now runs states -3, -2 and -1 before the current state each tick, after Ikemen GO's `actionRun` (`char.go:12497-12531`). On the reachable part of the collection (283 `.def` files: 67 ran before, 7 stopped by an engine error), the run now stops on an engine error for 53 characters and still moves none: the special states now execute and hit defects that were never reached before. The errors are the `VarSet` `var(N) = value` form (item 039; all 53 errors are this one message), and the characters that do run stay in their first state because their -1 triggers use `alive`, `ishelper` and unparsable expressions (items 028, 041), which the run neutralizes. Reaching more than one state therefore waits on 039, 041 and 028, not on 037 itself.

**Update, own-fighter triggers (item 028, first slice).** Same 283 `.def` files: `alive`, `ishelper`, `roundstate`, `prevstateno` and `random` (the five most exposed unknown triggers there, 67, 24, 13, 10 and 8 characters) are no longer unknown. 62 characters now stop on an engine error (53 before), all of them on the `VarSet` `var(N) = value` form (item 039): with these triggers known, more controllers run and reach it. 12 run to the end and still none leaves its first state. The most exposed unknown names left are `const()` (20 characters; its argument, e.g. `movement.stand.friction.threshold`, is also counted as a trigger name), `animelem`, `animelemtime()`, `fvar()` and `palno` (item 042).

Backlog items opened from these figures: engine 037 (special states), 038 (command matcher crash), 039 (`VarSet` form), 040 (`PlaySnd` common sounds), 041 (expression grammar); character 074 (`[Statedef]` header text), 075 (`.air` tolerance), 076 (`[State]` before `[Statedef]`), 077 (every state file in `Load`, `stcommon` fallback). Missing files and the truncated sprite are content defects, not tracked.

**Update, movement and state controllers (item 027).** `VelSet`, `VelAdd`, `VelMul`, `PosSet`, `PosAdd`, `ChangeAnim`, `CtrlSet`, `StateTypeSet`, `Turn`, `LifeAdd`, `LifeSet`, `Gravity` and `Null` are supported, so they no longer appear as unsupported types, and their `x`, `y`, `elem`, `elemtime` and `kill` parameters are checked for unknown triggers like `value` and `v`. A type the engine skips is also reported live, by name, in each tick's result (`unsupported`), see `.vibe/decisions/020`.

**Update, expression grammar (item 041).** The evaluator now accepts intervals, `**`, the bitwise operators, `^^` and `Cond`, after Ikemen GO's expression compiler. On the reference fixtures with `-common testdata/reference`: `kfm.def` went from 36 controllers with an unparsable trigger to 0; `kfm_zss.def` went from a tick error at tick 0 (an interval in `Function Combo`) to running its 3600 ticks. Every trigger of `common1.cns`, `kfm.cns`, `kfm.cmd` and every expression of the reference `.zss` files now parses (tests `TestParse_ReferenceStateFileTriggers_AllParse`, `TestCompile_ReferenceScriptExpressions_AllParse`). Trigger names they use that the engine still does not know: `BackEdgeBodyDist`, `BackEdgeDist`, `FrontEdgeBodyDist`, `GameTime`, `InGuardDist`, `MatchOver`, `MugenVersion()`, `Map()`, `Name`, `RoundsExisted`, `StateTime`, `TeamSide`, and the names `aiw` and `major` read inside calls. The collection-wide figure (349,244) has not been re-measured.
