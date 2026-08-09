# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [Unreleased]

## [0.2.0] - 2026-08-09

### Added

- The engine can now parse and evaluate MUGEN CNS trigger/expression syntax — comparisons, boolean and arithmetic operators, and built-in triggers such as `Time`, `Ctrl`, `Anim`, `Command`, `var()`, `sysvar()`, and `IfElse()` — against a fighter's live state, returning a clear error instead of a wrong or default result for malformed or unsupported expressions.

## [0.1.0] - 2026-08-09

### Added

- The engine now models the live state of a match while two characters fight — each fighter's position, facing, movement, and current state, plus the round number and round timer — as the foundation later combat-simulation features build on.

[Unreleased]: https://github.com/openkakutou/engine/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/openkakutou/engine/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/openkakutou/engine/releases/tag/v0.1.0
