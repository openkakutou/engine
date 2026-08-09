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
