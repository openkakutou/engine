package statemachine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/match"
)

// Movement, animation, control, state-type and life controllers. Each is
// grounded in Ikemen GO's src/bytecode.go (the controller's Run method) and
// src/char.go (the Char method it calls), cited per handler below.
//
// Coordinate conventions, see .vibe/decisions/020: Ikemen GO stores X
// relative to the fighter's facing (positive is forward) and Y pointing
// down; the engine stores X in world space and Y pointing up
// (match.Position, physics.Step). Every handler converts at the boundary,
// so a content author's "VelSet y = -8" is an upward 8 here.

// DefaultMaxHealth is MUGEN's default [Data] life; a character's own life
// caps LifeAdd and LifeSet (MaxHealth).
const DefaultMaxHealth = 1000

// invisibleAnim is Ikemen GO's "no sprite" animation number, the only
// negative number ChangeAnim keeps (char.go changeAnim).
const invisibleAnim = -2

func init() {
	RegisterController(ControllerTypeVelSet, applyVelSet)
	RegisterController(ControllerTypeVelAdd, applyVelAdd)
	RegisterController(ControllerTypeVelMul, applyVelMul)
	RegisterController(ControllerTypePosSet, applyPosSet)
	RegisterController(ControllerTypePosAdd, applyPosAdd)
	RegisterController(ControllerTypeChangeAnim, applyChangeAnim)
	RegisterController(ControllerTypeCtrlSet, applyCtrlSet)
	RegisterController(ControllerTypeStateTypeSet, applyStateTypeSet)
	RegisterController(ControllerTypeTurn, applyTurn)
	RegisterController(ControllerTypeLifeAdd, applyLifeAdd)
	RegisterController(ControllerTypeLifeSet, applyLifeSet)
	RegisterController(ControllerTypeGravity, applyGravity)
	RegisterController(ControllerTypeNull, applyNull)
	RegisterController(ControllerTypePlayerPush, applyPlayerPush)
}

// worldX converts an Ikemen GO facing-relative X to the engine's world-space
// X (char.go addX: x*c.facing). Adding to zero normalizes a negative zero,
// which would otherwise serialize as "-0" in JSON.
func worldX(x float64, f match.Facing) float64 {
	if f == match.FacingLeft {
		return 0 - x
	}
	return 0 + x
}

// noNegativeZero returns v, with a negative zero turned into +0.
func noNegativeZero(v float64) float64 {
	return 0 + v
}

// evalOptional evaluates ctrl's parameter key as an expression, reporting
// ok false when the parameter is absent.
func evalOptional(ctrl cns.Controller, key string, ctx *evaluator.Context) (value float64, ok bool, err error) {
	raw, present := ctrl.Parameters[key]
	if !present {
		return 0, false, nil
	}
	v, err := evaluator.EvaluateMut(raw, ctx)
	if err != nil {
		return 0, false, fmt.Errorf("%s %s %q: %w", ctrl.Type, key, raw, err)
	}
	return v.Float(), true, nil
}

// evalRequired is evalOptional for a parameter the controller cannot run
// without (Ikemen GO's compiler marks it required).
func evalRequired(ctrl cns.Controller, key string, ctx *evaluator.Context) (float64, error) {
	value, ok, err := evalOptional(ctrl, key, ctx)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, fmt.Errorf("%s is missing its required %q parameter", ctrl.Type, key)
	}
	return value, nil
}

// axes is a controller's optional "x" and "y" parameters, in Ikemen GO's
// units: an omitted axis is left untouched by the controller.
type axes struct {
	x, y       float64
	hasX, hasY bool
}

func evalAxes(ctrl cns.Controller, ctx *evaluator.Context) (axes, error) {
	var a axes
	var err error
	if a.x, a.hasX, err = evalOptional(ctrl, "x", ctx); err != nil {
		return axes{}, err
	}
	if a.y, a.hasY, err = evalOptional(ctrl, "y", ctx); err != nil {
		return axes{}, err
	}
	return a, nil
}

// worldY converts an Ikemen GO Y (down is positive) to the engine's Y (up
// is positive). Written as a subtraction from zero so a zero input stays
// +0 instead of becoming -0, which would serialize as "-0" in JSON.
func worldY(y float64) float64 {
	return 0 - y
}

// applyVelSet sets the velocity on each given axis. Ikemen GO bytecode.go
// velSet.Run: crun.vel[0|1] = value, with vel[0] facing-relative.
func applyVelSet(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	a, err := evalAxes(ctrl, &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}
	if a.hasX {
		ctx.Velocity.X = worldX(a.x, ctx.Facing)
	}
	if a.hasY {
		ctx.Velocity.Y = worldY(a.y)
	}
	return ctx, nil
}

// applyVelAdd adds to the velocity on each given axis. Ikemen GO
// bytecode.go velAdd.Run: crun.vel[0|1] += value.
func applyVelAdd(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	a, err := evalAxes(ctrl, &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}
	if a.hasX {
		ctx.Velocity.X += worldX(a.x, ctx.Facing)
	}
	if a.hasY {
		ctx.Velocity.Y -= a.y
	}
	return ctx, nil
}

// applyVelMul multiplies the velocity on each given axis. Ikemen GO
// bytecode.go velMul.Run: crun.vel[0|1] *= value. A factor is
// direction-free, so the world-space X needs no facing conversion.
func applyVelMul(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	a, err := evalAxes(ctrl, &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}
	if a.hasX {
		ctx.Velocity.X = noNegativeZero(ctx.Velocity.X * a.x)
	}
	if a.hasY {
		ctx.Velocity.Y = noNegativeZero(ctx.Velocity.Y * a.y)
	}
	return ctx, nil
}

// applyPosSet moves the fighter to the given coordinates. Ikemen GO
// bytecode.go posSet.Run sets x to sys.cam.Pos[0] plus the value and y to
// the value plus the ground level; the engine has no camera and its ground
// is Y = 0, so the value is taken as the stage coordinate (see
// .vibe/decisions/020).
func applyPosSet(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	a, err := evalAxes(ctrl, &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}
	if a.hasX {
		ctx.Position.X = noNegativeZero(a.x)
	}
	if a.hasY {
		ctx.Position.Y = worldY(a.y)
	}
	return ctx, nil
}

// applyPosAdd moves the fighter by the given offsets. Ikemen GO
// bytecode.go posAdd.Run calls char.go addX (x*facing) and addY.
func applyPosAdd(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	a, err := evalAxes(ctrl, &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}
	if a.hasX {
		ctx.Position.X += worldX(a.x, ctx.Facing)
	}
	if a.hasY {
		ctx.Position.Y -= a.y
	}
	return ctx, nil
}

// applyChangeAnim switches to animation "value" and restarts it. Ikemen GO
// bytecode.go changeAnim.Run calls char.go changeAnim (a negative number
// other than -2 becomes 0) and then, when "elem" or "elemtime" was given,
// setAnimElem, whose invalid element (below 1) becomes 1 with time 0. The
// element is only recorded here as a request on the Context: resolving it
// needs the animation's frames, which the root package's Tick has.
func applyChangeAnim(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	value, err := evalRequired(ctrl, "value", &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}
	elem, hasElem, err := evalOptional(ctrl, "elem", &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}
	elemTime, hasElemTime, err := evalOptional(ctrl, "elemtime", &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}

	anim := int(value)
	if anim < 0 && anim != invisibleAnim {
		anim = 0
	}
	ctx.Anim = anim
	ctx.AnimFromOpponent = false // its own animation (ChangeAnim2 sets it back)
	ctx.AnimElapsed = 0
	ctx.AnimElem, ctx.AnimElemTime = 0, 0
	if hasElem || hasElemTime {
		ctx.AnimElem, ctx.AnimElemTime = int(elem), int(elemTime)
		if ctx.AnimElem < 1 {
			ctx.AnimElem, ctx.AnimElemTime = 1, 0
		}
	}
	return ctx, nil
}

// applyCtrlSet gives or takes the fighter's control. Ikemen GO bytecode.go
// ctrlSet.Run: setSCF(SCF_ctrl, value).
func applyCtrlSet(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	value, err := evalRequired(ctrl, "value", &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}
	ctx.Ctrl = value != 0
	return ctx, nil
}

// applyStateTypeSet changes the state type and move type. Ikemen GO
// bytecode.go stateTypeSet.Run converts each parameter by its first letter,
// case-insensitively (stateTypeValue, moveTypeValue, physicsTypeValue) and
// treats U as "unchanged". It is also the only controller that sets the
// move type: Ikemen GO has no MoveTypeSet. The physics type sets
// Context.Physics, which decides whether Tick applies gravity (see
// .vibe/decisions/020).
func applyStateTypeSet(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	if raw, ok := ctrl.Parameters["statetype"]; ok {
		letter, err := typeLetter(ctrl, "statetype", raw, "SCALU")
		if err != nil {
			return evaluator.Context{}, err
		}
		if letter != 'U' {
			ctx.StateType = letter
		}
	}
	if raw, ok := ctrl.Parameters["movetype"]; ok {
		letter, err := typeLetter(ctrl, "movetype", raw, "IAHU")
		if err != nil {
			return evaluator.Context{}, err
		}
		if letter != 'U' {
			ctx.MoveType = letter
		}
	}
	if raw, ok := ctrl.Parameters["physics"]; ok {
		letter, err := typeLetter(ctrl, "physics", raw, "SCANU")
		if err != nil {
			return evaluator.Context{}, err
		}
		if letter != 'U' {
			ctx.Physics = letter
		}
	}
	return ctx, nil
}

// typeLetter reads raw's first letter, upper-cased, accepting it when it is
// one of allowed. Surrounding quotes (a .zss string) are ignored.
func typeLetter(ctrl cns.Controller, key, raw, allowed string) (byte, error) {
	s := strings.Trim(strings.TrimSpace(raw), `"`)
	if s == "" || !strings.Contains(allowed, strings.ToUpper(s[:1])) {
		return 0, fmt.Errorf("%s %s %q must start with one of %s", ctrl.Type, key, raw, allowed)
	}
	return strings.ToUpper(s[:1])[0], nil
}

// applyTurn flips the facing. Ikemen GO bytecode.go turn.Run calls
// char.go setFacing(-facing), which also negates its facing-relative vel[0]
// so the world-space velocity is unchanged -- already the case here, where
// Velocity.X is stored in world space.
func applyTurn(_ cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	if ctx.Facing == match.FacingRight {
		ctx.Facing = match.FacingLeft
		return ctx, nil
	}
	ctx.Facing = match.FacingRight
	return ctx, nil
}

// applyLifeAdd adds "value" to the fighter's health. Ikemen GO bytecode.go
// lifeAdd.Run calls char.go lifeAdd: "kill" defaults to true and, when
// false, damage that would reach 0 leaves 1; the result is clamped to
// [0, max] by lifeSet. "absolute" only decides whether the fighter's defense
// scales the value; the engine has no defense, so it never matters here.
// A fighter already at 0 health stays there: Ikemen GO keeps its KO flag
// through lifeAdd, while the engine reads KO from health alone, so adding
// life to a KO'd fighter must not revive it (see .vibe/decisions/020).
func applyLifeAdd(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	value, err := evalRequired(ctrl, "value", &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}
	kill, hasKill, err := evalOptional(ctrl, "kill", &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}
	canKill := !hasKill || kill != 0

	if ctx.Health <= 0 {
		return ctx, nil
	}
	health := ctx.Health + int(value)
	if !canKill && health <= 0 {
		health = 1
	}
	ctx.Health = clampHealth(health, &ctx)
	return ctx, nil
}

// applyLifeSet sets the fighter's health, clamped to [0, max]. Ikemen GO
// bytecode.go lifeSet.Run calls char.go lifeSet: Clamp(life, 0, lifeMax).
// A KO'd fighter stays KO'd, as in applyLifeAdd.
func applyLifeSet(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	value, err := evalRequired(ctrl, "value", &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}
	if ctx.Health <= 0 {
		return ctx, nil
	}
	ctx.Health = clampHealth(int(value), &ctx)
	return ctx, nil
}

// clampHealth keeps health within 0 and the character's life (lifeMax,
// its [Data] life, char.go:4615-4630).
func clampHealth(health int, ctx *evaluator.Context) int {
	return min(max(health, 0), MaxHealth(ctx))
}

// MaxHealth is the fighter's life, its [Data] life constant (1000 by
// default).
func MaxHealth(ctx *evaluator.Context) int {
	return ctx.Consts.Lookup("data.life").Int()
}

// applyGravity accelerates the fighter downward by its yaccel constant.
// Ikemen GO bytecode.go gravity.Run calls char.go gravity: vel[1] += yaccel
// (Y down there, so the engine subtracts, as physics.Step does).
func applyGravity(_ cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	ctx.Velocity.Y -= ctx.YAccel
	return ctx, nil
}

// applyNull does nothing, like Ikemen GO's Null controller.
func applyNull(_ cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	return ctx, nil
}

// reportUnsupported records that a controller of type t (lower-cased) fired
// without an implementation, once per type.
func reportUnsupported(ctx *evaluator.Context, t string) {
	if t == hitDefKey || slices.Contains(ctx.Unsupported, t) {
		return
	}
	ctx.Unsupported = append(ctx.Unsupported, t)
}

// TakeUnsupported returns the types reported on ctx (nil when none) and
// empties ctx.Unsupported while keeping its backing array for reuse.
func TakeUnsupported(ctx *evaluator.Context) []string {
	reported := ctx.Unsupported
	ctx.Unsupported = reported[:0]
	if len(reported) == 0 {
		return nil
	}
	return reported
}

var hitDefKey = strings.ToLower(ControllerTypeHitDef)

// lowerTypes memoizes strings.ToLower by the controller type as written, so
// dispatching a controller every tick does not allocate a lower-cased copy
// (the same per-tick reparse-avoidance convention as playSndValueCache). It
// only grows by the distinct spellings of controller types in loaded
// characters. Unsynchronized for the same single-threaded reason.
var lowerTypes = make(map[string]string)

func lowerType(controllerType string) string {
	if lowered, ok := lowerTypes[controllerType]; ok {
		return lowered
	}
	lowered := strings.ToLower(controllerType)
	lowerTypes[controllerType] = lowered
	return lowered
}

// applyPlayerPush sets the push properties for the current tick. Ikemen GO
// bytecode.go:11309-11327 (playerPush.Run): value sets the playerpush flag,
// priority the push priority, affectteam E/F/B gives 1/-1/0; its compiler
// asks for at least one parameter (compiler_functions.go:3546-3590). Tick
// resets the three before every unfrozen tick (evaluator.Context.PlayerPush).
func applyPlayerPush(ctrl cns.Controller, ctx evaluator.Context) (evaluator.Context, error) {
	value, hasValue, err := evalOptional(ctrl, "value", &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}
	priority, hasPriority, err := evalOptional(ctrl, "priority", &ctx)
	if err != nil {
		return evaluator.Context{}, err
	}
	raw, hasTeam := ctrl.Parameters["affectteam"]
	if !hasValue && !hasPriority && !hasTeam {
		return evaluator.Context{}, fmt.Errorf("%s needs at least one of value, priority, affectteam", ctrl.Type)
	}
	if hasTeam {
		letter, err := typeLetter(ctrl, "affectteam", raw, "EFB")
		if err != nil {
			return evaluator.Context{}, err
		}
		switch letter {
		case 'E':
			ctx.PushAffectTeam = 1
		case 'F':
			ctx.PushAffectTeam = -1
		default:
			ctx.PushAffectTeam = 0
		}
	}
	if hasValue {
		ctx.PlayerPush = value != 0
	}
	if hasPriority {
		ctx.PushPriority = int(priority)
	}
	return ctx, nil
}
