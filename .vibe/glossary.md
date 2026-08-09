# Ubiquitous Language

## Match state
The live state of a match while two characters fight: the round number, the round timer, and both fighters' fighter state. It is pure data — no evaluation, execution, or simulation logic lives on it; every other `engine` package reads from and writes to it each simulation tick. Built through a validating constructor that requires exactly one fighter per side, rather than allowing a malformed match state (wrong fighter count, or two fighters on the same side) to exist.
**Do not confuse with:** Fighter state (one fighter's own state, held inside a match state).
_Sources: `match/state.go`_

## Fighter state
One fighter's live state during a match: position, facing, velocity, current state number, and health. Its state number references a state defined by the fighter's loaded character data (a `cns.StateDef.Number`), but this package does not resolve that reference itself — later packages (evaluator, state machine) do.
**Do not confuse with:** Match state (the two-fighter, round-level state a fighter state lives inside of).
_Sources: `match/state.go`_

## Side
Which of the two match slots a fighter state occupies — `SideP1` or `SideP2`. A match state always holds exactly one fighter state per side.
_Sources: `match/state.go`_

## Facing
The horizontal direction a fighter currently faces — `FacingRight` or `FacingLeft`.
_Sources: `match/state.go`_
