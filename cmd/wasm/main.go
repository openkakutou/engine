//go:build js && wasm

// Command wasm is the WASM entrypoint for the engine library: thin
// syscall/js glue exposing enough of engine's simulation loop for a
// mode-* game app to drive a match, without a Go toolchain of its own —
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

	"github.com/openkakutou/engine"
	"github.com/openkakutou/engine/input"
	"github.com/openkakutou/engine/match"
	"github.com/openkakutou/engine/round"
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
				out = envelope(nil, fmt.Errorf("recovered from panic: %v", r))
			}
		}()

		data, err := fn(args)
		if err != nil {
			return envelope(nil, err)
		}
		encoded, err := json.Marshal(data)
		if err != nil {
			return envelope(nil, fmt.Errorf("encoding result as JSON: %w", err))
		}
		return envelope(encoded, nil)
	}
}

// envelope builds this module's uniform {data, error} JS return shape.
// Exactly one field is ever non-null.
func envelope(data []byte, err error) map[string]any {
	if err != nil {
		return map[string]any{"data": nil, "error": err.Error()}
	}
	return map[string]any{"data": string(data), "error": nil}
}

// argString extracts args[0] as a Go string, the JSON request payload
// every exposed function takes exactly one of.
func argString(args []js.Value) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("expected exactly 1 argument (a JSON string), got %d", len(args))
	}
	return args[0].String(), nil
}

// newMatchRequest is OpenKakutouEngine.newMatch's JSON request shape.
// Programs/Starting are indexed [P1, P2], matching match.Side's own
// SideP1/SideP2 ordering.
type newMatchRequest struct {
	Programs    [2]engine.FighterProgram `json:"programs"`
	Starting    [2]match.FighterState    `json:"starting"`
	RoundTimer  int                      `json:"roundTimer"`
	BestOf      int                      `json:"bestOf"`
	Bounds      stage.StageBoundaries    `json:"bounds"`
	Gravity     float64                  `json:"gravity"`
	ComboWindow int                      `json:"comboWindow"`
}

// newMatchResponse is OpenKakutouEngine.newMatch's JSON success payload.
type newMatchResponse struct {
	MatchID  int              `json:"matchId"`
	State    match.MatchState `json:"state"`
	Progress round.Progress   `json:"progress"`
}

// newMatch is OpenKakutouEngine.newMatch(requestJSON) as seen from JS:
// builds round 1 of a new match from both fighters' loaded character data
// and starting position/state/health, and returns an opaque match ID for
// tick/resetRound to operate on going forward.
func newMatch(args []js.Value) (any, error) {
	raw, err := argString(args)
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

	p1Runtime, err := engine.NewFighterRuntime(req.Starting[match.SideP1], req.Programs[match.SideP1].States)
	if err != nil {
		return nil, fmt.Errorf("side %v: %w", match.SideP1, err)
	}
	p2Runtime, err := engine.NewFighterRuntime(req.Starting[match.SideP2], req.Programs[match.SideP2].States)
	if err != nil {
		return nil, fmt.Errorf("side %v: %w", match.SideP2, err)
	}

	id := nextID
	nextID++
	sessions[id] = &session{
		programs:    req.Programs,
		runtimes:    [2]engine.FighterRuntime{match.SideP1: p1Runtime, match.SideP2: p2Runtime},
		bounds:      &req.Bounds,
		gravity:     req.Gravity,
		comboWindow: req.ComboWindow,
		state:       *state,
		progress:    progress,
	}

	return newMatchResponse{MatchID: id, State: *state, Progress: progress}, nil
}

// tickRequest is OpenKakutouEngine.tick's JSON request shape.
type tickRequest struct {
	MatchID int                `json:"matchId"`
	Inputs  [2]input.TickInput `json:"inputs"`
}

// tickResponse is OpenKakutouEngine.tick's JSON success payload.
type tickResponse struct {
	State       match.MatchState  `json:"state"`
	Round       round.RoundResult `json:"round"`
	Progress    round.Progress    `json:"progress"`
	MatchOver   bool              `json:"matchOver"`
	MatchWinner match.Side        `json:"matchWinner"`
}

// tickJS is OpenKakutouEngine.tick(requestJSON) as seen from JS: advances
// matchId's session by exactly one simulation tick and reports the
// resulting state, this tick's round outcome (if any), and updated
// best-of-N progress. A round outcome other than OutcomeNone is recorded
// into the session's Progress automatically -- the caller does not also
// call anything else to register it -- but the session's own MatchState
// is not reset for the next round on its own; the caller drives that via
// resetRound once it has decided to.
func tickJS(args []js.Value) (any, error) {
	raw, err := argString(args)
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
	result, err := engine.Tick(sess.state, sess.programs, sess.runtimes, req.Inputs, sess.bounds, sess.gravity, sess.tick, sess.comboWindow)
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
	State match.MatchState `json:"state"`
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
	raw, err := argString(args)
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
	raw, err := argString(args)
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

	p1Runtime, err := engine.NewFighterRuntime(req.Starting[match.SideP1], sess.programs[match.SideP1].States)
	if err != nil {
		return nil, fmt.Errorf("side %v: %w", match.SideP1, err)
	}
	p2Runtime, err := engine.NewFighterRuntime(req.Starting[match.SideP2], sess.programs[match.SideP2].States)
	if err != nil {
		return nil, fmt.Errorf("side %v: %w", match.SideP2, err)
	}

	sess.state = *state
	sess.runtimes = [2]engine.FighterRuntime{match.SideP1: p1Runtime, match.SideP2: p2Runtime}
	sess.tick = 0

	return resetRoundResponse{State: sess.state}, nil
}
