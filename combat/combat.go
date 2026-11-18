// Package combat applies hitdetect's HitEvents to actual match state:
// damage subtracted from the defender's health (never below zero) and a
// per-defender combo counter tracked across simulation ticks. Damage comes
// from a MUGEN HitDef state controller's raw, unevaluated "damage" string
// parameter -- interpreted numerically here for the first time in the
// backlog. This package takes only that string (DamageParams), not the
// full character/cns.Controller a HitDef is parsed into; the caller
// extracts it, keeping this package decoupled from a controller shape it
// otherwise never reads. This package does not drive any state-machine
// transition (e.g. a hit-reaction state) --
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

	"github.com/openkakutou/engine/hitdetect"
	"github.com/openkakutou/engine/match"
)

// DefaultDamage is applied when a HitDef's own "damage" parameter is
// missing or not a valid integer, rather than crashing or applying an
// undefined amount.
const DefaultDamage = 0

// DamageParams is the subset of a HitDef controller's parameters ApplyHits
// actually reads: its raw, unevaluated "damage" string. Narrower than the
// full cns.Controller ApplyHits used to take, decoupling this package from
// a controller shape it doesn't need -- unlike statemachine.ApplyController
// and zssexec's own controller-call handling, which genuinely need the
// whole cns.Controller because they dispatch across multiple controller
// types, ApplyHits is already scoped by its caller to exactly one type
// (HitDef) and reads exactly one field. See this item's own backlog entry.
type DamageParams struct {
	// Damage is the HitDef's raw "damage" parameter, unevaluated. Empty
	// when the parameter was missing from the controller -- parseDamage
	// falls back to DefaultDamage for both a missing and an unparseable
	// value, so the zero value here is a valid "no damage declared" input,
	// not a special case callers need to guard against.
	Damage string
}

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
	Hit      bool
	Defender match.Side
	Damage   int
	// GuardDamage is the HitDef's second damage value, kept for guarding.
	GuardDamage  int
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
	hitDef DamageParams,
	combo ComboState,
	tick, comboWindow int,
) (match.MatchState, ComboState, HitResult) {
	defender, landed := findLandedHit(events, attacker)
	if !landed {
		return state, combo, HitResult{}
	}

	damage, guardDamage := parseDamage(hitDef)

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
		GuardDamage:  guardDamage,
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

// parseDamage reads hitDef's Damage field as "hit[, guard]" (MUGEN 1.1 and
// Ikemen GO). Each value that is missing or not a valid integer falls back
// to DefaultDamage independently, so "15" and "15," give guard damage 0.
func parseDamage(hitDef DamageParams) (hit, guard int) {
	first, second := hitDef.Damage, ""
	if i := strings.IndexByte(first, ','); i >= 0 {
		first, second = first[:i], first[i+1:]
	}
	return atoiOrDefaultDamage(first), atoiOrDefaultDamage(second)
}

func atoiOrDefaultDamage(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		// Atoi allocates its error: keep the hit path allocation-free.
		return DefaultDamage
	}
	value, err := strconv.Atoi(raw)
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

// Defaults for the HitDef parameters ParseHitDef reads, from the MUGEN 1.1
// state-controller documentation: pausetime 0,0; ground.hittime 0;
// ground.slidetime 0; air.hittime 20; animtype Light; ground.type High;
// air.type = ground.type.
const (
	animTypeBack      = 3
	animTypeUp        = 4
	defaultAirHitTime = 20
	defaultGroundType = 1
	// Ikemen GO's HitDef defaults (char.go:730-885, HitDef.reset): yaccel
	// 0.35, fall.yvelocity -4.5, fall.recover on, fall.recovertime 4.
	defaultYAccel          = 0.35
	defaultFallYVelocity   = -4.5
	defaultFallRecoverTime = 4
)

// HitDefParams is the subset of a HitDef controller's parameters that drive
// hit pause and the defender's get-hit variables. AnimType and the two
// types carry MUGEN's numeric codes (animtype Light=0, Medium=1, Hard=2,
// Back=3, Up=4, DiagUp=5; type None=0, High=1, Low=2, Trip=3), the values
// GetHitVar reports. combat does not import evaluator: the root package
// maps this to evaluator.GetHit.
type HitDefParams struct {
	// PauseTime is {attacker freeze, defender shake} in ticks.
	PauseTime       [2]int
	GroundHitTime   int
	GroundSlideTime int
	AirHitTime      int
	AnimType        int
	GroundType      int
	AirType         int
	// AirAnimType and FallAnimType are air.animtype and fall.animtype, with
	// Ikemen GO's defaults (char.go:958-967), in the same codes as AnimType.
	AirAnimType, FallAnimType int
	// GroundVelocityY and AirVelocityY are the y of ground.velocity and
	// air.velocity, in the content's units (Y down); the x is not used here.
	GroundVelocityY, AirVelocityY float64
	// YAccel is the defender's gravity while it is knocked about (yaccel).
	YAccel float64
	// GroundFall and AirFall are fall and air.fall; air.fall follows fall when
	// absent (Ikemen GO finalizeParams).
	GroundFall, AirFall bool
	// FallYVelocity, FallRecover and FallRecoverTime are fall.yvelocity,
	// fall.recover and fall.recovertime.
	FallYVelocity   float64
	FallRecover     bool
	FallRecoverTime int
	// HitID is the HitDef's id, the one NumTarget(id) filters on.
	HitID int
}

// ParseHitDef reads a HitDef controller's raw, lower-cased parameter map
// (as character/cns stores it). Every value must be a plain integer or a
// known enum name; anything else -- a missing parameter, or an expression
// such as "var(0)" -- falls back to that parameter's default, the same
// stance parseDamage takes (documented divergence: MUGEN evaluates
// expressions here, see .vibe/decisions/018). Allocation-free: it runs on
// the hit path of the per-tick hot loop.
func ParseHitDef(params map[string]string) HitDefParams {
	p := HitDefParams{
		AirHitTime:      defaultAirHitTime,
		GroundType:      defaultGroundType,
		YAccel:          parseFloatOr(params["yaccel"], defaultYAccel),
		FallYVelocity:   parseFloatOr(params["fall.yvelocity"], defaultFallYVelocity),
		FallRecover:     parseIntOr(params["fall.recover"], 1) != 0,
		FallRecoverTime: parseIntOr(params["fall.recovertime"], defaultFallRecoverTime),
		HitID:           parseIntOr(params["id"], 0),
	}
	if raw, ok := params["pausetime"]; ok {
		first, second := raw, ""
		if i := strings.IndexByte(raw, ','); i >= 0 {
			first, second = raw[:i], raw[i+1:]
		}
		// A negative pause time is meaningless (and would leave a
		// defender shaking forever): clamp to 0.
		p.PauseTime[0] = max(parseIntOr(first, 0), 0)
		p.PauseTime[1] = max(parseIntOr(second, 0), 0)
	}
	p.GroundHitTime = parseIntOr(params["ground.hittime"], 0)
	p.GroundSlideTime = parseIntOr(params["ground.slidetime"], 0)
	p.AirHitTime = parseIntOr(params["air.hittime"], defaultAirHitTime)
	p.AnimType = animTypeCode(params["animtype"])
	p.AirAnimType = p.AnimType
	if raw, ok := params["air.animtype"]; ok {
		p.AirAnimType = animTypeCode(raw)
	}
	switch raw, ok := params["fall.animtype"]; {
	case ok:
		p.FallAnimType = animTypeCode(raw)
	case p.AirAnimType >= animTypeUp:
		p.FallAnimType = p.AirAnimType
	default:
		p.FallAnimType = animTypeBack
	}
	p.GroundType = hitTypeCode(params["ground.type"], defaultGroundType)
	p.AirType = hitTypeCode(params["air.type"], p.GroundType)
	p.GroundVelocityY = velocityY(params["ground.velocity"])
	p.AirVelocityY = velocityY(params["air.velocity"])
	p.GroundFall = parseIntOr(params["fall"], 0) != 0
	p.AirFall = parseIntOr(params["air.fall"], boolInt(p.GroundFall)) != 0
	return p
}

// velocityY is the y of an "x, y" velocity parameter, 0 when it has no y or
// the y is not a literal number.
func velocityY(raw string) float64 {
	i := strings.IndexByte(raw, ',')
	if i < 0 {
		return 0
	}
	return parseFloatOr(raw[i+1:], 0)
}

// parseFloatOr parses s as a decimal number, returning def when s is empty or
// not one. The empty check comes first so that a missing parameter, the common
// case, never reaches strconv's allocating error path.
func parseFloatOr(s string, def float64) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// parseIntOr parses s as a decimal integer (optional sign, surrounding
// space allowed), returning def when s is empty or not one -- an
// expression, for instance. Hand-rolled rather than strconv.Atoi, whose
// error path allocates a *NumError on every missing or non-literal value.
func parseIntOr(s string, def int) int {
	s = strings.TrimSpace(s)
	neg := false
	if s != "" && (s[0] == '-' || s[0] == '+') {
		neg = s[0] == '-'
		s = s[1:]
	}
	// Up to 9 digits cannot overflow an int on any target (WASM is 32-bit
	// ints in some toolchains); a longer value is not a sane tick count.
	if s == "" || len(s) > 9 {
		return def
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return def
		}
		n = n*10 + int(s[i]-'0')
	}
	if neg {
		return -n
	}
	return n
}

// firstLetter returns the upper-cased first non-space ASCII letter of s, or
// 0 when there is none. The enum parameters are decided from that letter
// alone (see .vibe/decisions/018: unverified against Ikemen GO's source).
func firstLetter(s string) byte {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' {
			continue
		}
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		return c
	}
	return 0
}

// animTypeCode maps an animtype to MUGEN's code from its first letter,
// case-insensitive (L 0, M 1, H 2, B 3, U 4, D 5), Light (0) when missing
// or unknown. "Heavy", as real characters (kfm.cns) write it, is H = Hard.
func animTypeCode(s string) int {
	switch firstLetter(s) {
	case 'M':
		return 1
	case 'H':
		return 2
	case 'B':
		return 3
	case 'U':
		return 4
	case 'D':
		return 5
	default:
		return 0
	}
}

// hitTypeCode maps a ground.type/air.type to MUGEN's code from its first
// letter, case-insensitive (N 0, H 1, L 2, T 3), def when missing or unknown.
func hitTypeCode(s string, def int) int {
	switch firstLetter(s) {
	case 'N':
		return 0
	case 'H':
		return 1
	case 'L':
		return 2
	case 'T':
		return 3
	default:
		return def
	}
}
