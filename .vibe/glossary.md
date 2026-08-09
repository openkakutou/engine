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

## Trigger
A MUGEN CNS expression — comparisons, boolean/arithmetic operators, and built-in names such as `Time`, `Ctrl`, or `Command` — checked against a fighter's live state to decide whether a state controller should act. `character/cns` stores triggers as unevaluated strings (`Controller.Triggers`); `engine/evaluator` is where a trigger string is parsed and evaluated to its actual bool/int/float result.
**Do not confuse with:** Command (one specific trigger, checking recognized input rather than fighter state).
_Sources: `evaluator/parser.go`, `evaluator/eval.go`_

## Command (input)
A named input pattern (e.g. `"holdback"`) that a fighter's currently recognized/held inputs are checked against by the `Command` trigger (`Command = "holdback"`). Real input-buffer matching against `.cmd` command definitions is future work (item 008); until then, the set of currently-recognized command names is supplied directly as `Context.ActiveCommands`.
**Do not confuse with:** Trigger (the general expression category `Command` is one specific instance of).
_Sources: `evaluator/eval.go`, `evaluator/context.go`_
