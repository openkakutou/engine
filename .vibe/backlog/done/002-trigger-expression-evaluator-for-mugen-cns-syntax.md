---
status: done
depends_on: [001]
---
# Trigger/Expression Evaluator For MUGEN CNS Syntax

## Description
This is the big missing piece the whole rest of the backlog builds on. `character/cns` deliberately leaves a state controller's triggers and parameters as unevaluated raw strings (`Controller.Triggers []string`, `Controller.Parameters map[string]string`) — see `character`'s own decision record on that split. `engine` is where those strings actually get evaluated. Implement a parser and evaluator for the MUGEN CNS expression language: comparisons (`=`, `!=`, `<`, `>`, `<=`, `>=`), boolean/arithmetic operators (`&&`, `||`, `+`, `-`, `*`, `/`, `%`, unary `!`), and the built-in trigger functions/variables a real character's `.cns` relies on (at minimum `var()`, `sysvar()`, `IfElse`, `Time`, `StateNo`, `Anim`, `AnimTime`, `Command`, `Ctrl`, `HitDefAttr`-style helpers as needed — extend incrementally, not exhaustively, in this item). Evaluation reads from the `MatchState`/fighter state defined in item 001 and returns a typed result (bool/int/float, matching MUGEN's loose typing rules for triggers). "Done" for this item means: a representative sample of real trigger strings pulled from actual character `.cns` files parses and evaluates to the expected value against a hand-built `MatchState` fixture — not just synthetic expressions invented for the test suite.

## Acceptance Criteria
- [x] Expression strings for comparisons and arithmetic/boolean operators parse into an evaluable representation, honoring MUGEN operator precedence
- [x] Evaluation against a `MatchState`/fighter state fixture returns the correct value for each supported operator and at least the built-in functions/variables listed above
- [x] A malformed or unparseable expression string returns a descriptive error rather than panicking
- [x] A well-formed expression referencing an unsupported/unknown trigger name returns a descriptive error rather than silently evaluating to a default
- [x] Fixture-driven tests evaluate real trigger strings sourced from actual MUGEN/Ikemen GO `.cns` files, not only hand-written expressions

## Notes
None.
