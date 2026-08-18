// Package engine is the root package of OpenKakutou's combat simulation
// library.
//
// It will hold the match/combat state model, MUGEN CNS trigger evaluator,
// state-machine execution, .zss script execution, physics, hit detection,
// damage/combo resolution, and round/match flow — see CLAUDE.md for the
// full scope and its explicit boundary (combat simulation only, not menus
// or game flow).
package engine

// Version is the current version of the engine module.
//
// It exists as a minimal, buildable skeleton for the repo's bootstrap —
// real functionality is added incrementally per the backlog.
const Version = "0.4.0"
