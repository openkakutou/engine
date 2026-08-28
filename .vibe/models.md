# Data models

## FighterState
| Field | Type | Notes |
|---|---|---|
| Side | `Side` (int enum) | Which match slot (`SideP1`/`SideP2`) this fighter occupies; zero value is `SideP1` |
| Position | `Position` | 2D stage coordinates; zero value is the origin |
| Facing | `Facing` (int enum) | `FacingRight` (zero value) or `FacingLeft` |
| Velocity | `Velocity` | Per-axis movement rate, applied to `Position` by later physics |
| StateNo | `int` | References a loaded character's `cns.StateDef.Number`; not resolved by this package |
| Health | `int` | Remaining health points |
Defined in: `match/state.go`

## MatchState
| Field | Type | Notes |
|---|---|---|
| Round | `int` | Current round number; 0 in the zero value |
| RoundTimer | `int` | Remaining round time, in simulation ticks |
| Fighters | `[2]FighterState` | Indexed by `Side`; use `Fighter(side)` for side-keyed access rather than indexing directly |
Built via: `NewMatchState(round, roundTimer int, fighters ...FighterState) (*MatchState, error)` — requires exactly one `FighterState` per `Side`, returns a descriptive error otherwise (wrong count, or two fighters sharing the same `Side`).
Defined in: `match/state.go`

## Position / Velocity
| Field | Type | Notes |
|---|---|---|
| X | `float64` | |
| Y | `float64` | |
Both are plain value types with a valid, usable zero value (`{0, 0}`).
Defined in: `match/state.go`

## Context
| Field | Type | Notes |
|---|---|---|
| FighterState (embedded) | `match.FighterState` | Gives access to `StateNo` and the other item-001 fields |
| Time | `int` | Ticks elapsed since entering the current `StateNo`; populated by future state-machine execution (item 003) |
| Ctrl | `bool` | Whether the fighter currently has control |
| Anim | `int` | Current animation number |
| AnimTime | `int` | Ticks elapsed since `Anim` started playing |
| Vars | `[60]int` | MUGEN general-purpose variable slots, indexed by `var(n)`; unset index reads as 0 |
| SysVars | `[5]int` | MUGEN system variable slots, indexed by `sysvar(n)`; unset index reads as 0 |
| ActiveCommands | `map[string]bool` | Command names currently recognized, checked by the `Command` trigger; populated from `input.Step`'s return value; a nil map reads every command as inactive |
Zero value is valid and usable (reads as a fighter that just entered state 0 with no input). See `.vibe/decisions/002` for why this is a separate type from `match.FighterState`.
Defined in: `evaluator/context.go`

## Value / Kind
| Field | Type | Notes |
|---|---|---|
| Kind | `Kind` (int enum: `KindBool`, `KindInt`, `KindFloat`) | What produced the value — a comparison/logical op, an int literal/arithmetic, or a float literal/arithmetic |
| Number | `float64` | The value itself; a bool is stored as 1 or 0 |
`Bool()`, `Float()`, `Int()` expose `Number` as the type a caller needs, matching MUGEN's loose typing (`true` = `1`, `3` = `3.0`).
Defined in: `evaluator/eval.go`

## Result
| Field | Type | Notes |
|---|---|---|
| Context | `evaluator.Context` | The fighter's context after applying every controller that triggered true during one `Step` call |
| Applied | `[]int` | Declared-order indices, into the current state's `Controllers` slice, of every controller whose trigger conditions evaluated true this call — present even if the controller's type has no implemented effect |
Returned by: `Step(ctx evaluator.Context, states map[int]cns.StateDef) (Result, error)`
Defined in: `statemachine/statemachine.go`

## zssexec.Result
| Field | Type | Notes |
|---|---|---|
| Context | `evaluator.Context` | The fighter's context after running the current Statedef's `.zss` script body for one `Step` call |
Unlike `statemachine.Result`, there is no `Applied` field: a `.zss` body is imperative control flow (`if`/`call`/controller statements), not a flat, independently-triggered controller list, so there is no equivalent per-controller "did its trigger hold" list to report.
Returned by: `Step(ctx evaluator.Context, script zss.Script) (Result, error)`
Defined in: `zssexec/zssexec.go`

## TickInput
| Field | Type | Notes |
|---|---|---|
| Up, Down, Left, Right | `bool` | Raw held directions for this tick — not facing-relative; `input.Step` resolves them against `match.Facing` internally |
| Buttons | `map[string]bool` | Currently-held button names (e.g. `"a"`); a nil map reads every button as not held |
Zero value (nothing held) is valid and usable.
Defined in: `input/matcher.go`

## ComboState
| Field | Type | Notes |
|---|---|---|
| Count | `int` | Consecutive hits landed within `comboWindow` of each other so far |
| LastHitTick | `int` | The simulation tick the most recent hit landed on |
| Active | `bool` | Whether Count/LastHitTick are meaningful yet; false on the zero value and after `ResetCombo()` |
Threaded by the caller across ticks, one instance per defender, the same way `input.State` is threaded through `input.Step`.
Defined in: `combat/combat.go`

## HitResult
| Field | Type | Notes |
|---|---|---|
| Hit | `bool` | Whether any event matched the attacking side this call |
| Defender | `match.Side` | Only meaningful when Hit is true |
| Damage | `int` | The amount actually subtracted (post-`DefaultDamage`-fallback) |
| HealthBefore / HealthAfter | `int` | Defender's health before/after this hit; HealthAfter is floored at 0 |
| ComboCount | `int` | The defender's combo count after this hit |
Returned by: `combat.ApplyHits(...)` — a zero `HitResult` when no event matched the attacking side.
Defined in: `combat/combat.go`

## HitEvent
| Field | Type | Notes |
|---|---|---|
| Attacker | `match.Side` | Which fighter's Clsn1 (hit) box overlapped |
| Defender | `match.Side` | Which fighter's Clsn2 (hurt) box overlapped |
| AttackerBox | `air.ClsnBox` | The original local box, not the transformed world-space one compared |
| DefenderBox | `air.ClsnBox` | Same as above, for the defender's box |
Returned by: `hitdetect.Detect(state *match.MatchState, frames [2]air.Frame) []HitEvent` — one per overlapping box pair found, across both attack directions.
Defined in: `hitdetect/hitdetect.go`
