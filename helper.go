package engine

import (
	"fmt"
	"github.com/openkakutou/engine/v2/statemachine"

	"github.com/openkakutou/character/air"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/input"
	"github.com/openkakutou/engine/v2/match"
)

// helperMax is Ikemen GO's default HelperMax (defaultConfig.ini), the most
// helpers a player can have (char.go:7172-7178).
const helperMax = 56

// Helper is one of a player's helpers (Ikemen GO helper chars): a fighter
// running the player's own states with its own runtime, made by a Helper
// controller and gone on DestroySelf or the round's end.
type Helper struct {
	FighterRuntime
	// Serial tells the player's helpers apart for life; Parent is the
	// spawner's serial, 0 for the player.
	Serial, Parent int
	// Frame is the helper's animation frame this tick, its hit boxes.
	Frame air.Frame
}

// helperTick is what a player's helpers did this tick, merged into the
// player's own report.
type helperTick struct {
	sounds   []evaluator.SoundEvent
	warnings []string
}

// tickHelpers spawns the helpers the player made this tick, then runs every
// helper, new ones included, as Ikemen GO's charList.action runs them after
// the players in creation order (char.go:7214-7247 actionPrepare); helpers
// they spawn run in the same tick too. A destroyed helper is dropped. It
// leaves the player's Helpers snapshots for NumHelper and helper(id).
// shortcut: helpers cannot be hit, pushed or screen-bound; a keyctrl
// helper matches the player's commands with its own buffer, from its spawn.
func tickHelpers(state *match.MatchState, rt *FighterRuntime, prog FighterProgram, opponent evaluator.Opponent, in input.TickInput, cfg TickConfig, screen [2]float64, round int, seed *int32) (helperTick, error) {
	var out helperTick
	// A fresh list: the snapshot slice of the tick before belongs to it.
	rt.Context.Helpers = helperSnapshots(nil, rt.Helpers)
	out.warnings = spawnHelpers(rt, &rt.Context, 0, prog, opponent, out.warnings)
	for i := 0; i < len(rt.Helpers); i++ {
		h := &rt.Helpers[i]
		// A helper without move time stands frozen through a pause.
		if pausedFor(state, h.SuperMoveTime, h.PauseMoveTime) {
			continue
		}
		h.Context.Root = snapshotOf(rt.Context.FighterState, &rt.Context)
		h.Context.Parent = h.Context.Root
		if parent := findHelper(rt.Helpers, h.Parent); parent != nil {
			h.Context.Parent = snapshotOf(parent.Context.FighterState, &parent.Context)
		}
		h.Context.Helpers = rt.Context.Helpers
		h.Context.RoundState, h.Context.RoundEnd = rt.Context.RoundState, rt.Context.RoundEnd
		// A helper with keyctrl reads the player's input, so its commands are
		// the player's (char.go:5672); the others read none.
		var helperIn input.TickInput
		if h.Context.KeyCtrl[0] {
			helperIn = in
		}
		res, err := tickFighter(fighterPrograms{prog, prog}, h.FighterRuntime, h.Context.FighterState, opponent, helperIn, cfg.Tick, cfg.Bounds, screen, cfg.Gravity, round, *seed)
		if err != nil {
			return out, fmt.Errorf("helper %d: %w", h.Context.HelperID, err)
		}
		*seed = res.Runtime.Context.RandSeed
		h.FighterRuntime, h.Frame = res.Runtime, res.Frame
		if err := applyParentVarOps(rt, h); err != nil {
			return out, fmt.Errorf("helper %d: %w", h.Context.HelperID, err)
		}
		bindHelper(rt, h)
		armHitDef(&h.FighterRuntime, res)
		out.sounds = append(out.sounds, res.Sounds...)
		out.warnings = append(out.warnings, res.Warnings...)
		rt.Context.Events = append(rt.Context.Events, h.Context.Events...)
		spawnProjectiles(&h.Context, rt)
		spawner := h.Context
		h.Context.NewHelpers = nil
		out.warnings = spawnHelpers(rt, &spawner, h.Serial, prog, opponent, out.warnings)
	}
	kept := rt.Helpers[:0]
	for _, h := range rt.Helpers {
		if !h.Context.Destroyed {
			kept = append(kept, h)
		}
	}
	rt.Helpers = kept
	rt.Context.Helpers = helperSnapshots(nil, rt.Helpers)
	return out, nil
}

// spawnHelpers turns spawner's Helper controllers of this tick into the
// player's helpers (newHelper, helperInit, char.go:7156-7247): in its state
// stateno at pos from postype (p1, or p2 the opponent; others are read as
// p1), facing its way or, with a negative facing, away; life at the
// player's maximum. Past helperMax, or into a state the character lacks, it
// warns and makes none.
// shortcut: postype front, back, left and right are read as p1; a missing
// state is not entered empty as Ikemen GO does.
func spawnHelpers(rt *FighterRuntime, spawner *evaluator.Context, serial int, prog FighterProgram, opponent evaluator.Opponent, warnings []string) []string {
	for _, s := range spawner.NewHelpers {
		if len(rt.Helpers) >= helperMax {
			warnings = append(warnings, fmt.Sprintf("Reached limit of %d helpers. Helper creation skipped", helperMax))
			continue
		}
		if !prog.hasState(s.StateNo) {
			warnings = append(warnings, fmt.Sprintf("helper %d: state %d not found, helper creation skipped", s.ID, s.StateNo))
			continue
		}
		origin, facing := spawner.Position, spawner.Facing
		if s.PosType == "p2" && opponent.Valid {
			origin, facing = opponent.Position, opponent.Facing
		}
		pos := match.Position{X: origin.X + s.Pos[0]*facingSign(facing), Y: origin.Y - s.Pos[1]}
		if s.Facing < 0 {
			facing = 1 - facing
		}
		life := rt.Context.Consts.Lookup("data.life").Int()
		if life <= 0 {
			life = rt.Context.Health
		}
		fighter := match.FighterState{Side: rt.Context.Side, Position: pos, Facing: facing, Health: life, StateNo: s.StateNo}
		hrt, err := prog.NewRuntime(fighter)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("helper %d: %v", s.ID, err))
			continue
		}
		hrt.Context.IsHelper, hrt.Context.HelperID, hrt.Context.KeyCtrl = true, s.ID, s.KeyCtrl
		hrt.PauseMoveTime, hrt.SuperMoveTime = s.PauseMoveTime, s.SuperMoveTime
		hrt.Context.AILevel = rt.Context.AILevel
		hrt.Context.IkemenChar, hrt.Context.MugenVersion = rt.Context.IkemenChar, rt.Context.MugenVersion
		rt.NextHelper++
		rt.Helpers = append(rt.Helpers, Helper{FighterRuntime: hrt, Serial: rt.NextHelper, Parent: serial})
		// The new helper counts at once for NumHelper and helper(id)
		// (char.go:6162-6177 reads sys.chars, which newHelper appended to).
		rt.Context.Helpers = append(rt.Context.Helpers, snapshotOf(hrt.Context.FighterState, &hrt.Context))
	}
	spawner.NewHelpers = nil
	return warnings
}

func findHelper(helpers []Helper, serial int) *Helper {
	for i := range helpers {
		if helpers[i].Serial == serial {
			return &helpers[i]
		}
	}
	return nil
}

// helperSnapshots is the player's helpers as its triggers see them, in dst's
// storage.
func helperSnapshots(dst []evaluator.Opponent, helpers []Helper) []evaluator.Opponent {
	for i := range helpers {
		dst = append(dst, snapshotOf(helpers[i].Context.FighterState, &helpers[i].Context))
	}
	return dst
}

// hitWithHelpers lands the attacker's helpers' HitDefs whose hit boxes touch
// the defender's hurt boxes, each like the player's own, the first one only
// (a defender is hit once a tick). The helper, not the player, gets the hit
// pause and the contact.
func hitWithHelpers(state *match.MatchState, programs *[2]FighterProgram, runtimes *[2]FighterRuntime, attacker match.Side, defClsn2 []air.ClsnBox, cfg TickConfig, hitSounds *[2][]evaluator.SoundEvent) error {
	rt := &runtimes[attacker]
	defender := &runtimes[1-attacker].Context
	for i := range rt.Helpers {
		h := &rt.Helpers[i]
		if !hitDefReady(&h.FighterRuntime) {
			continue
		}
		if !hitBoxesTouch(h.Context.Position, h.Context.Facing, h.Frame.Clsn1, defender, defClsn2) {
			continue
		}
		if !evaluator.HittableBy(defender.HitBy, h.Context.HitDefAttr) {
			continue
		}
		owner := ownerContact{rt.HitPause, rt.Context.MoveContactTime, rt.Context.ContactGuarded, rt.Context.HasTarget}
		hit, guarded, params := strike(state, runtimes, attacker, h.HitDef, &h.Context, cfg, hitSounds)
		h.HitDefHit = hit.Hit
		if !hit.Hit {
			continue
		}
		err := applyHitReaction(programs, runtimes, attacker, params, hit, guarded)
		h.HitPause = rt.HitPause
		h.Context.MoveContactTime, h.Context.ContactGuarded, h.Context.HasTarget = 1, guarded, true
		rt.HitPause, rt.Context.MoveContactTime, rt.Context.ContactGuarded, rt.Context.HasTarget = owner.pause, owner.moveContact, owner.guarded, owner.target
		return err
	}
	return nil
}

// applyParentVarOps writes the helper's ParentVarSet and ParentVarAdd of the
// tick to its parent: the player, or the helper that spawned it.
func applyParentVarOps(rt *FighterRuntime, h *Helper) error {
	ops := h.Context.ParentVarOps
	h.Context.ParentVarOps = nil
	if len(ops) == 0 {
		return nil
	}
	parent := &rt.Context
	if p := findHelper(rt.Helpers, h.Parent); p != nil {
		parent = &p.Context
	}
	for _, op := range ops {
		if err := statemachine.ApplyVarOp(parent, op); err != nil {
			return err
		}
	}
	return nil
}

// bindHelper holds a bound helper on its parent or root (bindToPlayer,
// char.go:10262-10312): at its position plus pos, x toward its facing,
// facing its way or away when the bind asks, then counts the bind down.
func bindHelper(rt *FighterRuntime, h *Helper) {
	b := &h.Context.Bind
	if b.Time == 0 {
		return
	}
	to := &rt.Context
	if p := findHelper(rt.Helpers, h.Parent); p != nil && !b.Root {
		to = &p.Context
	}
	h.Context.Position = match.Position{X: to.Position.X + b.Pos[0]*facingSign(to.Facing), Y: to.Position.Y - b.Pos[1]}
	switch b.Facing {
	case 1:
		h.Context.Facing = to.Facing
	case -1:
		h.Context.Facing = 1 - to.Facing
	}
	if b.Time > 0 {
		b.Time--
	}
}
