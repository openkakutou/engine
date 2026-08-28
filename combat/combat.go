// Package combat applies hitdetect's HitEvents to actual match state:
// damage subtracted from the defender's health (never below zero) and a
// per-defender combo counter tracked across simulation ticks. Damage comes
// from a MUGEN HitDef state controller's raw, unevaluated "damage" string
// parameter (character/cns's Controller.Parameters) -- interpreted
// numerically here for the first time in the backlog. This package does
// not drive any state-machine transition (e.g. a hit-reaction state) --
// no acceptance criterion of the item that introduced it asked for that,
// and it would require state-machine integration out of this item's own
// scope. See .vibe/decisions/010-combat-hit-dedup-scope-and-allocation-discipline.md
// for that scoping call, why multiple overlapping Clsn box pairs against
// the same defender count as one landed hit, and why this hot path (run
// twice per simulation tick, once per attacking side, for the whole match)
// holds to a zero-allocation, by-value-state discipline.
package combat

import (
	"strconv"
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/hitdetect"
	"github.com/openkakutou/engine/match"
)

// DefaultDamage is applied when a HitDef's own "damage" parameter is
// missing or not a valid integer, rather than crashing or applying an
// undefined amount.
const DefaultDamage = 0

// ComboState tracks one fighter's (the defender's) combo progress across
// simulation ticks -- threaded by the caller between calls to ApplyHits,
// the same shape input.State is threaded through input.Step. Its zero
// value is a fresh, inactive combo (no hit landed yet).
type ComboState struct {
	// Count is the number of consecutive hits landed within comboWindow of
	// each other so far.
	Count int
	// LastHitTick is the simulation tick the most recent hit landed on.
	LastHitTick int
	// Active reports whether Count/LastHitTick are meaningful yet -- false
	// on the zero value and immediately after ResetCombo.
	Active bool
}

// HitResult reports what one ApplyHits call did. Hit is false, and every
// other field its zero value, when no event in that call matched the
// attacking side.
type HitResult struct {
	Hit          bool
	Defender     match.Side
	Damage       int
	HealthBefore int
	HealthAfter  int
	ComboCount   int
}

// ApplyHits applies every hit event in events attributed to attacker to
// state, using hitDef's raw "damage" parameter, at the given tick.
// hitdetect.Detect checks both attack directions in one call; a caller
// invokes ApplyHits once per attacking side, passing that side's own
// currently active HitDef controller.
//
// Multiple events against the same defender in one call (several
// overlapping Clsn box pairs from the same attack) count as a single
// landed hit, not one hit per pair -- see this package's own decision
// record. combo is the defender's combo state entering this tick;
// comboWindow is the number of ticks since the last hit within which a
// further hit continues the same combo rather than starting a fresh one
// at count 1. Health never drops below zero.
//
// Returns the updated MatchState, the defender's updated ComboState, and
// the HitResult -- state and combo are returned unchanged, and a zero
// HitResult, when no event in events matches attacker.
func ApplyHits(
	state match.MatchState,
	events []hitdetect.HitEvent,
	attacker match.Side,
	hitDef cns.Controller,
	combo ComboState,
	tick, comboWindow int,
) (match.MatchState, ComboState, HitResult) {
	defender, landed := findLandedHit(events, attacker)
	if !landed {
		return state, combo, HitResult{}
	}

	damage := parseDamage(hitDef)

	defenderState := state.Fighter(defender)
	healthBefore := defenderState.Health
	healthAfter := healthBefore - damage
	if healthAfter < 0 {
		healthAfter = 0
	}
	defenderState.Health = healthAfter
	state.Fighters[defender] = defenderState

	newCombo := advanceCombo(combo, tick, comboWindow)

	return state, newCombo, HitResult{
		Hit:          true,
		Defender:     defender,
		Damage:       damage,
		HealthBefore: healthBefore,
		HealthAfter:  healthAfter,
		ComboCount:   newCombo.Count,
	}
}

// ResetCombo returns a fresh, inactive ComboState -- the caller's own
// signal (e.g. it determined the defender's state changed to a recovered/
// non-hit one) that any in-progress combo has ended, independent of the
// time-based window ApplyHits enforces on its own.
func ResetCombo() ComboState {
	return ComboState{}
}

// findLandedHit reports the defender of the first event in events
// attributed to attacker, and whether one was found at all. A plain
// linear scan with an early return -- no set/map -- since a call is
// already scoped to one attacker and stops at the first match.
func findLandedHit(events []hitdetect.HitEvent, attacker match.Side) (match.Side, bool) {
	for _, e := range events {
		if e.Attacker == attacker {
			return e.Defender, true
		}
	}
	return 0, false
}

// parseDamage reads hitDef's "damage" parameter as an integer, falling
// back to DefaultDamage when it's missing or not a valid integer.
func parseDamage(hitDef cns.Controller) int {
	raw, ok := hitDef.Parameters["damage"]
	if !ok {
		return DefaultDamage
	}
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return DefaultDamage
	}
	return value
}

// advanceCombo returns combo advanced by one more landed hit at tick: a
// fresh count of 1 if combo wasn't active yet or the window since its last
// hit already lapsed, otherwise its count incremented by one.
func advanceCombo(combo ComboState, tick, comboWindow int) ComboState {
	if !combo.Active || tick-combo.LastHitTick > comboWindow {
		return ComboState{Count: 1, LastHitTick: tick, Active: true}
	}
	return ComboState{Count: combo.Count + 1, LastHitTick: tick, Active: true}
}
