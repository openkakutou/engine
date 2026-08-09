# Module: match

**Role:** Pure-data model for the live state of a match while two characters fight: per-fighter state (position, facing, velocity, current state number, health) and match-level state (round number, round timer, both fighters keyed by side). Every later `engine` package (trigger evaluator, state machine, physics, hit detection, damage/combo, round flow) is expected to read from and write to these types each simulation tick. Carries no evaluation, execution, or simulation logic itself, matching `character`'s established pattern of separating pure-data types from the logic that operates on them.
**Files:** `match/state.go`
**Exports:** `Side` (`SideP1`, `SideP2`), `Facing` (`FacingRight`, `FacingLeft`), `Position{X, Y float64}`, `Velocity{X, Y float64}`, `FighterState{Side, Position, Facing, Velocity, StateNo, Health}`, `MatchState{Round, RoundTimer, Fighters [2]FighterState}`, `NewMatchState(round, roundTimer int, fighters ...FighterState) (*MatchState, error)`, `(*MatchState) Fighter(side Side) FighterState`
**Depends on:** none (standard library `fmt` only)
