//go:build js && wasm

// Command wasm is the WASM entrypoint for the engine library: thin
// syscall/js glue exposing enough of engine's simulation loop for a
// `game` app to drive a match, without a Go toolchain of its own —
// following character's cmd/wasm precedent (build-tag-gated, a
// JS-callable global, verified by a Node.js smoke harness since
// syscall/js code cannot run under the plain Go toolchain, see smoke.mjs).
//
// Every exposed function takes and/or returns a single JSON-encoded
// string, and never throws — a Go panic is recovered and reported as
// {data: null, error: "..."} instead of tearing down the whole module. Per-
// match runtime state (FighterRuntime, input.State, combat.ComboState —
// the last two carry unexported fields by design and would silently lose
// data if round-tripped through JSON) stays Go-side in a session, keyed by
// an opaque match ID newMatch returns; tick/resetRound take that ID back
// and only ever hand JS already-JSON-tagged summary data (MatchState,
// RoundResult, Progress). closeMatch releases a session's state once the
// caller is done with it -- sessions are never pruned on their own. See
// ../../.vibe/decisions/011.
package main

import (
	"encoding/json"
	"fmt"
	"syscall/js"

	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/v2"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
	"github.com/openkakutou/engine/v2/round"
	"github.com/openkakutou/stage"
)

func main() {
	js.Global().Set("OpenKakutouEngine", js.ValueOf(map[string]any{
		"newMatch":   js.FuncOf(guarded(newMatch)),
		"tick":       js.FuncOf(guarded(tickJS)),
		"resetRound": js.FuncOf(guarded(resetRoundJS)),
		"closeMatch": js.FuncOf(guarded(closeMatchJS)),
	}))

	// Registering js.FuncOf callbacks does not keep the Go runtime alive on
	// its own; block forever so OpenKakutouEngine keeps working for the
	// lifetime of the page.
	select {}
}

// session is one in-progress match's Go-resident runtime state -- never
// serialized to JSON, never leaves this module.
type session struct {
	programs    [2]engine.FighterProgram
	runtimes    [2]engine.FighterRuntime
	bounds      *stage.StageBoundaries
	gravity     float64
	comboWindow int
	tick        int
	state       match.MatchState
	progress    round.Progress
}

var (
	sessions = map[int]*session{}
	nextID   = 1
)

// guarded wraps fn so a panic anywhere inside it (a malformed request this
// module's own JSON decoding didn't already catch, an out-of-range slice
// access, etc.) is recovered and reported as this module's normal
// {data: null, error: "..."} shape instead of propagating out of the
// js.Func callback and tearing down the whole page's WASM instance -- the
// same boundary responsibility character/cmd/wasm's own load/
// resolveSprites already take on themselves.
func guarded(fn func(args []js.Value) (any, error)) func(js.Value, []js.Value) any {
	return func(this js.Value, args []js.Value) (out any) {
		defer func() {
			if r := recover(); r != nil {
				out = buildEnvelope(nil, fmt.Errorf("recovered from panic: %v", r))
			}
		}()

		data, err := fn(args)
		if err != nil {
			return buildEnvelope(nil, err)
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			return buildEnvelope(nil, fmt.Errorf("encoding result as JSON: %w", err))
		}
		return buildEnvelope(encoded, nil)
	}
}

// buildEnvelope builds this module's uniform {data, error} JS return shape.
// Exactly one field is ever non-null.
func buildEnvelope(data []byte, err error) map[string]any {
	if err != nil {
		return map[string]any{"data": nil, "error": err.Error()}
	}
	return map[string]any{"data": string(data), "error": nil}
}

// extractArgString extracts args[0] as a Go string, the JSON request payload
// every exposed function takes exactly one of.
func extractArgString(args []js.Value) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("expected exactly 1 argument (a JSON string), got %d", len(args))
	}
	return args[0].String(), nil
}

// programRequest is one fighter's program as the JS side sends it: the
// engine.FighterProgram fields (flattened by the embedding) plus the
// character's compiled-at-load .zss scripts in load order (Scripts, each a
// character/zss Script). A state number defined in both `states` and
// `scripts` resolves to `states`; to get Ikemen GO's first-file-wins across
// the two, the JS side leaves out of `states` the numbers an earlier script
// defines (engine.NewFighterProgramFromFiles does it in Go).
type programRequest struct {
	engine.FighterProgram
	Scripts []zss.Script `json:"scripts"`
}

// build returns the FighterProgram with its animation lookup and its .zss
// scripts compiled once, at match creation. FighterProgram's own JSON
// decoding only ever populates its exported States/Animations/Commands
// fields, never the unexported index and compiled script (see
// .vibe/decisions/013 and 024).
func (r programRequest) build() engine.FighterProgram {
	return engine.NewFighterProgram(r.States, r.Animations, r.Commands).WithScripts(r.Scripts)
}

// newMatchRequest is OpenKakutouEngine.newMatch's JSON request shape.
// Programs/Starting are indexed [P1, P2], matching match.Side's own
// SideP1/SideP2 ordering.
type newMatchRequest struct {
	Programs    [2]programRequest     `json:"programs"`
	Starting    [2]match.FighterState `json:"starting"`
	RoundTimer  int                   `json:"roundTimer"`
	BestOf      int                   `json:"bestOf"`
	Bounds      stage.StageBoundaries `json:"bounds"`
	Gravity     float64               `json:"gravity"`
	ComboWindow int                   `json:"comboWindow"`
	// RandSeed seeds the match's Random sequence (match.MatchState.RandSeed);
	// omitted, it is 0.
	RandSeed int32 `json:"randSeed"`
}

// FighterAnimState is one fighter's currently resolved animation number and
// elapsed frame timing -- additive-only, response-side data (see this
// item's own ADR, .vibe/decisions/012): sourced from the session's
// Go-resident FighterRuntime (Context.Anim/AnimElapsed, kept out of JSON on
// the request side per .vibe/decisions/011), never round-tripped back in.
// Named AnimNo, not Anim, to read unambiguously next to FighterState's own
// StateNo in the same response -- a fighter's animation number and its
// state number are related but not the same thing (a state's declared Anim
// can differ from its own state number via changeanim) and must not look
// like the same field twice.
type FighterAnimState struct {
	AnimNo   int `json:"animNo"`
	AnimTime int `json:"animTime"`
}

// animState builds sess's current per-side FighterAnimState, indexed by
// match.Side the same way as the request/response's own Fighters/Programs/
// Starting arrays -- shared by newMatch/tick/resetRound so all three
// responses populate this field identically.
func animState(sess *session) [2]FighterAnimState {
	var out [2]FighterAnimState
	out[match.SideP1] = FighterAnimState{AnimNo: sess.runtimes[match.SideP1].Context.Anim, AnimTime: sess.runtimes[match.SideP1].Context.AnimElapsed}
	out[match.SideP2] = FighterAnimState{AnimNo: sess.runtimes[match.SideP2].Context.Anim, AnimTime: sess.runtimes[match.SideP2].Context.AnimElapsed}
	return out
}

// newMatchResponse is OpenKakutouEngine.newMatch's JSON success payload.
type newMatchResponse struct {
	MatchID    int                 `json:"matchId"`
	State      match.MatchState    `json:"state"`
	Progress   round.Progress      `json:"progress"`
	Animations [2]FighterAnimState `json:"animations"`
}

// newMatch is OpenKakutouEngine.newMatch(requestJSON) as seen from JS:
// builds round 1 of a new match from both fighters' loaded character data
// and starting position/state/health, and returns an opaque match ID for
// tick/resetRound to operate on going forward.
func newMatch(args []js.Value) (any, error) {
	raw, err := extractArgString(args)
	if err != nil {
		return nil, err
	}

	var req newMatchRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return nil, fmt.Errorf("decoding request: %w", err)
	}

	progress, err := round.NewProgress(req.BestOf)
	if err != nil {
		return nil, err
	}

	state, err := match.NewMatchState(1, req.RoundTimer, req.Starting[match.SideP1], req.Starting[match.SideP2])
	if err != nil {
		return nil, err
	}
	state.RandSeed = req.RandSeed

	// Rebuilding each side's FighterProgram here, once at match creation,
	// precomputes its animation lookup and compiles its .zss scripts for the
	// whole match's worth of ticks that follow (see programRequest.build).
	programs := [2]engine.FighterProgram{
		match.SideP1: req.Programs[match.SideP1].build(),
		match.SideP2: req.Programs[match.SideP2].build(),
	}
	p1Runtime, err := programs[match.SideP1].NewRuntime(req.Starting[match.SideP1])
	if err != nil {
		return nil, fmt.Errorf("side %v: %w", match.SideP1, err)
	}
	p2Runtime, err := programs[match.SideP2].NewRuntime(req.Starting[match.SideP2])
	if err != nil {
		return nil, fmt.Errorf("side %v: %w", match.SideP2, err)
	}

	id := nextID
	nextID++
	sessions[id] = &session{
		programs:    programs,
		runtimes:    [2]engine.FighterRuntime{match.SideP1: p1Runtime, match.SideP2: p2Runtime},
		bounds:      &req.Bounds,
		gravity:     req.Gravity,
		comboWindow: req.ComboWindow,
		state:       *state,
		progress:    progress,
	}

	return newMatchResponse{MatchID: id, State: *state, Progress: progress, Animations: animState(sessions[id])}, nil
}

// tickRequest is OpenKakutouEngine.tick's JSON request shape.
type tickRequest struct {
	MatchID int                `json:"matchId"`
	Inputs  [2]input.TickInput `json:"inputs"`
}

// tickResponse is OpenKakutouEngine.tick's JSON success payload.
type tickResponse struct {
	State       match.MatchState    `json:"state"`
	Round       round.RoundResult   `json:"round"`
	Progress    round.Progress      `json:"progress"`
	MatchOver   bool                `json:"matchOver"`
	MatchWinner match.Side          `json:"matchWinner"`
	Animations  [2]FighterAnimState `json:"animations"`
	// Sounds is each fighter's PlaySnd events triggered this tick, indexed
	// by match.Side like Animations -- unlike Animations (current,
	// continuous state, safe to read late), this is a discrete per-tick
	// event log: a client that skips a tick's response loses that event
	// permanently, it is not recoverable from a later tick's own Sounds.
	// Each side is always a JSON array, normalized to `[]` rather than
	// `null` when empty (see normalizeSoundEvents) -- engine.TickResult's
	// own Go API keeps the idiomatic nil-slice-means-empty instead; see
	// .vibe/decisions/017.
	Sounds [2][]evaluator.SoundEvent `json:"sounds"`
	// Unsupported is each fighter's controller types that fired this tick
	// without an engine implementation, lower-cased and deduplicated,
	// indexed by match.Side. Each side is always a JSON array (`[]` when
	// none), so a client can tell "character runs fully" from "character
	// asks for something the engine skips". Additive: no existing field
	// changed. See .vibe/decisions/020.
	Unsupported [2][]string `json:"unsupported"`
	// SkippedCalls is each fighter's .zss calls that did nothing this tick
	// (an undefined function, a wrong argument or return count), indexed by
	// match.Side, each side always a JSON array like Unsupported. A parallel
	// field, not part of `unsupported`: those are controller types, these
	// are statements (decision 012's additive rule, decision 024).
	SkippedCalls [2][]string `json:"skippedCalls"`
}

// normalizeUnsupported (also used for skippedCalls) returns names unchanged if non-nil, or a non-nil
// empty slice otherwise, so the JSON field is `[]`, never `null`.
func normalizeUnsupported(names []string) []string {
	if names == nil {
		return []string{}
	}
	return names
}

// normalizeSoundEvents returns events unchanged if non-nil, or a non-nil
// empty slice otherwise -- so tickResponse.Sounds always marshals to a
// JSON array (`[]`), never `null`, for a side that triggered no PlaySnd
// controller this tick. engine.TickResult.Sounds itself stays a Go-idiomatic
// nil slice; this normalization is specific to this JSON/JS-facing
// boundary, mirroring character's own LoadBytes normalization guarantee
// (see character's .vibe/decisions/019) -- not retrofitted onto the Go API.
func normalizeSoundEvents(events []evaluator.SoundEvent) []evaluator.SoundEvent {
	if events == nil {
		return []evaluator.SoundEvent{}
	}
	return events
}

// tickJS is OpenKakutouEngine.tick(requestJSON) as seen from JS: advances
// matchId's session by exactly one simulation tick and reports the
// resulting state, this tick's round outcome (if any), updated best-of-N
// progress, and each fighter's sound events triggered this tick (see
// tickResponse.Sounds). A round outcome other than OutcomeNone is recorded
// into the session's Progress automatically -- the caller does not also
// call anything else to register it -- but the session's own MatchState
// is not reset for the next round on its own; the caller drives that via
// resetRound once it has decided to.
func tickJS(args []js.Value) (any, error) {
	raw, err := extractArgString(args)
	if err != nil {
		return nil, err
	}

	var req tickRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return nil, fmt.Errorf("decoding request: %w", err)
	}

	sess, ok := sessions[req.MatchID]
	if !ok {
		return nil, fmt.Errorf("no match session with id %d", req.MatchID)
	}

	sess.tick++
	cfg := engine.TickConfig{Bounds: sess.bounds, Gravity: sess.gravity, Tick: sess.tick, ComboWindow: sess.comboWindow}
	result, err := engine.Tick(sess.state, sess.programs, sess.runtimes, req.Inputs, cfg)
	if err != nil {
		return nil, err
	}

	sess.state = result.State
	sess.runtimes = result.Fighters
	if result.Round.Outcome != round.OutcomeNone {
		sess.progress = sess.progress.RecordRoundResult(result.Round)
	}

	matchOver, winner := sess.progress.MatchOutcome()

	return tickResponse{
		State:       sess.state,
		Round:       result.Round,
		Progress:    sess.progress,
		MatchOver:   matchOver,
		MatchWinner: winner,
		Animations:  animState(sess),
		Sounds: [2][]evaluator.SoundEvent{
			match.SideP1: normalizeSoundEvents(result.Sounds[match.SideP1]),
			match.SideP2: normalizeSoundEvents(result.Sounds[match.SideP2]),
		},
		Unsupported: [2][]string{
			match.SideP1: normalizeUnsupported(result.Unsupported[match.SideP1]),
			match.SideP2: normalizeUnsupported(result.Unsupported[match.SideP2]),
		},
		SkippedCalls: [2][]string{
			match.SideP1: normalizeUnsupported(result.SkippedCalls[match.SideP1]),
			match.SideP2: normalizeUnsupported(result.SkippedCalls[match.SideP2]),
		},
	}, nil
}

// resetRoundRequest is OpenKakutouEngine.resetRound's JSON request shape.
type resetRoundRequest struct {
	MatchID    int                   `json:"matchId"`
	RoundTimer int                   `json:"roundTimer"`
	Starting   [2]match.FighterState `json:"starting"`
}

// resetRoundResponse is OpenKakutouEngine.resetRound's JSON success
// payload.
type resetRoundResponse struct {
	State      match.MatchState    `json:"state"`
	Animations [2]FighterAnimState `json:"animations"`
}

// closeMatchRequest is OpenKakutouEngine.closeMatch's JSON request shape.
type closeMatchRequest struct {
	MatchID int `json:"matchId"`
}

// closeMatchJS is OpenKakutouEngine.closeMatch(requestJSON) as seen from
// JS: releases matchId's Go-resident session state. sessions is never
// pruned on its own -- a caller (a page driving many matches/rematches
// over its lifetime, e.g. after a match ends or the player backs out to a
// menu) is expected to call this once a given match ID is no longer
// needed, or that session's two FighterProgram/FighterRuntime pairs stay
// resident in memory for the life of the WASM instance.
func closeMatchJS(args []js.Value) (any, error) {
	raw, err := extractArgString(args)
	if err != nil {
		return nil, err
	}

	var req closeMatchRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return nil, fmt.Errorf("decoding request: %w", err)
	}

	if _, ok := sessions[req.MatchID]; !ok {
		return nil, fmt.Errorf("no match session with id %d", req.MatchID)
	}
	delete(sessions, req.MatchID)

	return map[string]any{}, nil
}

// resetRoundJS is OpenKakutouEngine.resetRound(requestJSON) as seen from
// JS: restores matchId's session to a fresh next round -- both fighters
// back to their given starting state/position/health, a fresh round
// timer, and each fighter's own runtime state (state-machine context,
// command-recognition progress, combo) reset the same way match start
// does. Progress (rounds won so far) is untouched -- only tick, via a
// recorded round outcome, ever advances it.
func resetRoundJS(args []js.Value) (any, error) {
	raw, err := extractArgString(args)
	if err != nil {
		return nil, err
	}

	var req resetRoundRequest
	if err := json.Unmarshal([]byte(raw), &req); err != nil {
		return nil, fmt.Errorf("decoding request: %w", err)
	}

	sess, ok := sessions[req.MatchID]
	if !ok {
		return nil, fmt.Errorf("no match session with id %d", req.MatchID)
	}

	nextRound := sess.state.Round + 1
	state, err := round.ResetRound(nextRound, req.RoundTimer, req.Starting[match.SideP1], req.Starting[match.SideP2])
	if err != nil {
		return nil, err
	}

	p1Runtime, err := sess.programs[match.SideP1].NewRuntime(req.Starting[match.SideP1])
	if err != nil {
		return nil, fmt.Errorf("side %v: %w", match.SideP1, err)
	}
	p2Runtime, err := sess.programs[match.SideP2].NewRuntime(req.Starting[match.SideP2])
	if err != nil {
		return nil, fmt.Errorf("side %v: %w", match.SideP2, err)
	}

	sess.state = *state
	sess.runtimes = [2]engine.FighterRuntime{match.SideP1: p1Runtime, match.SideP2: p2Runtime}
	sess.tick = 0

	return resetRoundResponse{State: sess.state, Animations: animState(sess)}, nil
}
