// Package engine is the root package of OpenKakutou's combat simulation
// library: the match/combat state model, MUGEN CNS trigger evaluator,
// state-machine execution, .zss script execution, physics, hit detection,
// damage/combo resolution, and round/match flow, tied into a usable
// per-tick match loop by Tick (see tick.go) — see CLAUDE.md for the full
// scope and its explicit boundary (combat simulation only, not menus or
// game flow).
package engine

// Version is the current version of the engine module.
const Version = "2.0.1"
