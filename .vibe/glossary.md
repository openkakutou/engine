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

## State controller
A single conditional action defined within one of a character's states (a `cns.Controller`, e.g. a state change or a variable assignment): while its state is active, its trigger is checked every simulation tick, and its effect is applied only when that trigger holds. `engine/statemachine` evaluates a state's controllers in the order the character file declares them, applying each one that triggers true and updating the fighter's live context as it goes, so a later controller in the same tick can observe an earlier one's effect. Once a controller changes the fighter's current state, no further controller in that state runs for the rest of that tick.
**Do not confuse with:** Trigger (the condition a state controller is gated on, not the action itself).
_Sources: `statemachine/statemachine.go`_

## Command (input)
A named input pattern (e.g. `"QCF_a"`) recognized when a fighter's raw per-tick input matches its `.cmd`-declared step sequence within a buffered time window; the `Command` trigger (`Command = "QCF_a"`) checks whether it is currently recognized via `Context.ActiveCommands`, which `input.Step` populates.
**Do not confuse with:** Trigger (the general expression category `Command` is one specific instance of).
_Sources: `evaluator/eval.go`, `evaluator/context.go`, `input/matcher.go`_

## Grounded
A fighter's state of resting on, or still falling toward, the stage's ground plane rather than being airborne: at or below ground level (`Position.Y <= 0`) and not moving upward (`Velocity.Y <= 0`). Gravity applies to a fighter only while it is not grounded, and landing — crossing back to the ground plane — zeroes its vertical velocity. A fighter given upward velocity while still at ground level is treated as airborne starting that same tick, not grounded.
**Do not confuse with:** Stage boundary (the horizontal range a fighter is clamped to, a separate constraint from the vertical ground plane).
_Sources: `physics/physics.go`_

## Hit box / hurt box
The two kinds of Clsn (collision) box a fighter's currently active animation frame can carry: a hit box (Clsn1) is a region that can land an attack on an opponent; a hurt box (Clsn2) is a region vulnerable to being hit. `engine/hitdetect` resolves a fighter's hit boxes against the other's hurt boxes each simulation tick to find overlaps.
**Do not confuse with:** Hit event (the *result* of a hit box overlapping a hurt box, not the boxes themselves).
_Sources: `hitdetect/hitdetect.go`_

## Hit event
One detected overlap between an attacker's hit box and a defender's hurt box for the current simulation tick: which fighter attacked, which fighter was hit, and which specific boxes overlapped. Reported by `engine/hitdetect`, consumed by `engine/combat` — detecting a hit and applying its effect (health loss, combo count) are deliberately separate concerns.
**Do not confuse with:** Hit box / hurt box (the geometry a hit event is computed from, not the event itself).
_Sources: `hitdetect/hitdetect.go`_

## Combo
A run of consecutive hits landed on the same defender close enough together in time (within the combo window) to count as one continuous sequence. Tracked as a running count that climbs while hits keep landing within the window and starts fresh at 1 the moment a hit lands after the window has lapsed.
**Do not confuse with:** Hit event (one landed hit; a combo is the running count of them).
_Sources: `combat/combat.go`_

## Round outcome
How a round ends: a knockout (one fighter's health reaches zero), a double knockout (both fighters reach zero health the same tick, awarding neither side a win), a timeout (the round timer expires, decided by whichever fighter has more health remaining), or a timeout draw (the round timer expires with both fighters at exactly equal health). A knockout is always decided before a timeout, so a fighter that runs out of health and out of time on the same tick is still reported as a knockout.
**Do not confuse with:** Match outcome (whether the whole best-of-N match, not just one round, has been won).
_Sources: `round/round.go`_

## Best-of-N match
A match played to a fixed, always-odd number of rounds, decided outright once either side has won more than half of them — so a match can never end in a tie at the round level. Each round's own outcome is recorded into a running count of rounds won per side; a drawn round (a double knockout or a timeout draw) advances the round count without awarding either side a win toward that total.
**Do not confuse with:** Round outcome (how one single round ends, the input a best-of-N match's own running tally is built from).
_Sources: `round/round.go`_
