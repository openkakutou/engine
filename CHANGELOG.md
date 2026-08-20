# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

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

[Unreleased]: https://github.com/openkakutou/engine/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/openkakutou/engine/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/openkakutou/engine/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/openkakutou/engine/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/openkakutou/engine/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/openkakutou/engine/releases/tag/v0.1.0
