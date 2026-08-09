// Package match defines the pure-data model for the live state of a match
// while two characters fight: per-fighter state (position, facing,
// velocity, and which state the fighter is currently in) and match-level
// state (round number, round timer, and which side each fighter occupies).
//
// This is the foundation every later engine package (trigger evaluator,
// state-machine execution, physics, hit detection, damage/combo, round
// flow) reads from and writes to every simulation tick. It carries no
// evaluation, execution, or simulation logic — matching character's
// established pattern of keeping pure-data types separate from the logic
// that operates on them.
package match

import "fmt"

// Side identifies which of the two fighters in a match a FighterState
// belongs to. Its zero value is SideP1, so a zero-value FighterState reads
// as "player 1's fighter" rather than an invalid/unset side.
type Side int

const (
	// SideP1 is the first fighter slot.
	SideP1 Side = iota
	// SideP2 is the second fighter slot.
	SideP2
)

// numSides is the number of fighters a MatchState always holds. A fighting
// match is inherently two-sided; this is not expected to change.
const numSides = 2

// Facing is the horizontal direction a fighter faces. Its zero value is
// FacingRight, matching MUGEN/Ikemen's convention that player 1 starts
// facing right.
type Facing int

const (
	// FacingRight is facing toward the positive X direction.
	FacingRight Facing = iota
	// FacingLeft is facing toward the negative X direction.
	FacingLeft
)

// Position is a fighter's 2D location in stage coordinates. Its zero value
// (the origin) is a valid, usable position.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Velocity is a fighter's per-axis rate of movement, in stage units per
// simulation tick. Its zero value (no movement) is valid and usable.
type Velocity struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// FighterState is the live, per-tick state of one fighter during a match.
//
// Its zero value is valid and usable: it reads as SideP1, standing at the
// origin, facing right, motionless, in state 0, with 0 health — later
// engine packages are expected to populate it from real character/round
// data rather than rely on the zero value for gameplay-correct defaults.
type FighterState struct {
	// Side is which of the two match slots this fighter occupies.
	Side Side `json:"side"`
	// Position is the fighter's current location in stage coordinates.
	Position Position `json:"position"`
	// Facing is the direction the fighter currently faces.
	Facing Facing `json:"facing"`
	// Velocity is the fighter's current per-axis movement rate, applied to
	// Position by later physics (engine's physics/movement item).
	Velocity Velocity `json:"velocity"`
	// StateNo identifies the fighter's current state, matching a loaded
	// character's cns.StateDef.Number — later engine packages (evaluator,
	// state machine) resolve this reference against that character's
	// loaded StateDefs; this package never resolves it itself.
	StateNo int `json:"stateNo"`
	// Health is the fighter's remaining health points.
	Health int `json:"health"`
}

// MatchState is the live state of a match between two fighters: the round
// number, the round timer, and both fighters' FighterState.
//
// Its zero value is valid and usable: Round and RoundTimer read as 0, and
// both fighter slots read as their own zero-value FighterState — reading
// either never panics, even before NewMatchState populates it.
type MatchState struct {
	// Round is the current round number (1-indexed in normal play; 0 in
	// the zero value, before a round has started).
	Round int `json:"round"`
	// RoundTimer is the round's remaining time, in simulation ticks.
	RoundTimer int `json:"roundTimer"`
	// Fighters holds both fighters' state, indexed by Side (Fighters[SideP1],
	// Fighters[SideP2]) — use Fighter for side-keyed access instead of
	// indexing directly.
	Fighters [numSides]FighterState `json:"fighters"`
}

// NewMatchState builds a MatchState for the given round and round timer
// from exactly two fighters, one per Side, placing each into its matching
// slot in the returned MatchState's Fighters regardless of argument order.
//
// It returns a descriptive error, rather than a malformed or
// panic-inducing MatchState, if fighters does not contain exactly two
// elements or if both elements declare the same Side.
func NewMatchState(round, roundTimer int, fighters ...FighterState) (*MatchState, error) {
	if len(fighters) != numSides {
		return nil, fmt.Errorf("match: NewMatchState requires exactly %d fighters, got %d", numSides, len(fighters))
	}

	var ms MatchState
	ms.Round = round
	ms.RoundTimer = roundTimer

	var seen [numSides]bool
	for _, f := range fighters {
		if f.Side < SideP1 || f.Side > SideP2 {
			return nil, fmt.Errorf("match: NewMatchState received a fighter with invalid Side %v", f.Side)
		}
		if seen[f.Side] {
			return nil, fmt.Errorf("match: NewMatchState received two fighters for the same Side %v", f.Side)
		}
		seen[f.Side] = true
		ms.Fighters[f.Side] = f
	}

	return &ms, nil
}

// Fighter returns the FighterState for the given Side. On a MatchState
// that was never populated by NewMatchState for that side (including a
// zero-value MatchState), it returns that slot's zero-value FighterState
// rather than panicking.
func (m *MatchState) Fighter(side Side) FighterState {
	return m.Fighters[side]
}
