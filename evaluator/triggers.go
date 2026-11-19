package evaluator

import (
	"fmt"
	"math"
	"strings"

	"github.com/openkakutou/engine/v2/match"
)

// axisTriggers are the triggers written with an axis letter ("Pos Y").
var axisTriggers = map[string]bool{"pos": true, "vel": true, "p2dist": true, "p2bodydist": true}

// randomMax is Ikemen GO's IMax (common.go), the generator's modulus.
const randomMax = 2147483647

// evalFighterTrigger evaluates the argument-free triggers that read only the
// fighter itself and the match it is in. ok is false when name is not one of
// them. Grounded in Ikemen GO's bytecode.go (commit 54d4148a) at the
// OC_life, OC_power, OC_alive, OC_facing, OC_prevstateno, OC_roundstate,
// OC_ex_roundno and OC_random cases.
func evalFighterTrigger(name string, ctx *Context) (v Value, ok bool, err error) {
	switch name {
	case "life":
		return intValue(float64(ctx.Health)), true, nil
	case "power":
		return intValue(float64(ctx.Power)), true, nil
	case "alive":
		// shortcut: Ikemen GO reads its KO flag, set when life reaches 0;
		// health > 0 is the same until KO handling (item 026 area) exists.
		return boolValue(ctx.Health > 0), true, nil
	case "facing":
		if ctx.Facing == match.FacingRight {
			return intValue(1), true, nil
		}
		return intValue(-1), true, nil
	case "prevstateno":
		return intValue(float64(ctx.PrevStateNo)), true, nil
	case "roundno":
		return intValue(float64(ctx.Round)), true, nil
	case "roundstate":
		// shortcut: Tick only runs while the round is being fought, which is
		// Ikemen GO's roundstate 2; pre-intro, over and post states return
		// when the round flow models them.
		return intValue(2), true, nil
	case "ishelper":
		// The engine has no helpers yet, so every fighter is a player.
		return boolValue(false), true, nil
	case "random":
		return intValue(float64(drawRandom(ctx))), true, nil
	case "ailevel":
		// Ikemen GO reads sys.aiLevel[playerNo] (bytecode.go:2510-2515,
		// char.go:5529-5537), 0 for a human player; every fighter here is
		// human-driven.
		return intValue(0), true, nil
	case "ailevelf":
		return floatValue(0), true, nil // bytecode.go:3654-3659
	case "palno":
		// bytecode.go:2671: the palette the game chose; 1 until it sets one
		// (char.go:4888-4913).
		if ctx.PalNo < 1 {
			return intValue(1), true, nil
		}
		return intValue(float64(ctx.PalNo)), true, nil
	case "p2stateno":
		return opponentInt(ctx, float64(ctx.P2.StateNo)), true, nil // bytecode.go:3341-3347
	case "p2life":
		return opponentInt(ctx, float64(ctx.P2.Health)), true, nil
	case "numenemy":
		// char.go:6002-6014: the enemies of a one-on-one match are the opponent.
		if ctx.P2.Valid {
			return intValue(1), true, nil
		}
		return intValue(0), true, nil
	case "numtarget":
		return numTarget(ctx, -1), true, nil
	case "movecontact":
		// char.go:5967-5973: |mctime| unless the last contact was a reversal.
		return intValue(float64(ctx.MoveContactTime)), true, nil
	case "movehit":
		return intValue(float64(ctx.MoveContactTime)), true, nil
	case "moveguarded", "movereversed":
		// char.go:5981-5999. shortcut: the engine has no guarding or ReversalDef
		// yet, so no contact is ever of these types.
		return intValue(0), true, nil
	case "hitfall":
		return boolValue(ctx.GetHit.Fall), true, nil
	case "canrecover":
		return boolValue(ctx.GetHit.FallRecover && ctx.GetHit.FallTime >= ctx.GetHit.FallRecoverTime), true, nil
	case "numhelper", "numexplod":
		// bytecode.go:2657-2660. shortcut: the engine has no helpers or
		// explods yet (backlog items 029 and 052), so there is none to count.
		return intValue(0), true, nil
	}
	return Value{}, false, nil
}

// opponentInt is an opponent value as an int Value, 0 with no opponent.
func opponentInt(ctx *Context, v float64) Value {
	if !ctx.P2.Valid {
		return intValue(0)
	}
	return intValue(v)
}

// numTarget is Char.numTarget (char.go:6211): 1 while the opponent is the
// fighter's target (and carries HitDef id when id is not negative), else 0.
func numTarget(ctx *Context, id int) Value {
	if isTarget(ctx, id) {
		return intValue(1)
	}
	return intValue(0)
}

// drawRandom advances ctx.RandSeed one step of Ikemen GO's generator
// (common.go Random: Park-Miller with Schrage's method) and returns
// Rand(0, 999).
func drawRandom(ctx *Context) int32 {
	seed := ctx.RandSeed
	w := seed / 127773
	seed = (seed-w*127773)*16807 - w*2836
	if seed <= 0 {
		seed += randomMax
		if seed == randomMax { // the seed was 0
			seed--
		}
	}
	ctx.RandSeed = seed
	return seed / (randomMax/1000 + 1)
}

// evalComponent evaluates "Pos X/Y/Z", "Vel X/Y/Z", "P2Dist X/Y/Z" and
// "P2BodyDist X/Y/Z".
func evalComponent(n componentNode, ctx *Context) (Value, error) {
	var x, y float64
	switch n.name {
	case "p2dist", "p2bodydist":
		return evalP2Distance(n, ctx)
	case "pos":
		x, y = ctx.Position.X, ctx.Position.Y
	case "vel":
		// Vel X is relative to facing: positive is forward. The engine's
		// Velocity.X is in world space; Ikemen GO moves by vel[0]*facing
		// (char.go:10109), so Vel X is the world value times the facing sign.
		x, y = ctx.Velocity.X, ctx.Velocity.Y
		if ctx.Facing == match.FacingLeft {
			x = -x
		}
	default:
		return Value{}, &UnknownTriggerError{Name: n.name}
	}
	switch n.axis {
	case 'x':
		return floatValue(x), nil
	case 'y':
		// MUGEN's Y axis points down, the engine's up (match.Position): a
		// fighter above the ground has a negative Pos Y.
		return floatValue(-y), nil
	default:
		return Value{}, fmt.Errorf("evaluator: %s Z is not supported: fighters have no depth", n.name)
	}
}

// evalFighterCall evaluates the function-style triggers: the math functions
// and IsHelper(...). ok is false when n.name is not one of them.
func evalFighterCall(n callNode, ctx *Context) (v Value, ok bool, err error) {
	switch n.name {
	case "ishelper":
		return boolValue(false), true, nil
	case "numtarget":
		// The optional id picks the targets hit by the HitDef with that id
		// (compiler.go:3440-3445, char.go:6211-6224); none means every target.
		if len(n.args) > 1 {
			return Value{}, true, fmt.Errorf("evaluator: numtarget() expects at most 1 argument, got %d", len(n.args))
		}
		id := -1
		for _, arg := range n.args {
			v, err := evalNode(arg, ctx)
			if err != nil {
				return Value{}, true, err
			}
			id = v.Int()
		}
		return numTarget(ctx, id), true, nil
	case "numhelper", "numexplod":
		// The optional id picks which helpers or explods to count
		// (compiler.go:3415-3425); with none to count it changes nothing.
		if len(n.args) > 1 {
			return Value{}, true, fmt.Errorf("evaluator: %s() expects at most 1 argument, got %d", n.name, len(n.args))
		}
		for _, arg := range n.args {
			if _, err := evalNode(arg, ctx); err != nil {
				return Value{}, true, err
			}
		}
		return intValue(0), true, nil
	case "animelemtime", "animelemno", "animexist", "selfanimexist":
		return evalAnimCall(n, ctx)
	case "const240p":
		return evalConstScale(n, ctx, 320)
	case "const480p":
		return evalConstScale(n, ctx, 640)
	case "const720p":
		return evalConstScale(n, ctx, 1280)
	case "const1080p":
		return evalConstScale(n, ctx, 1920)
	case "abs":
		return evalUnaryMath(n, ctx, func(a Value) (Value, error) {
			if a.Kind == KindFloat {
				return floatValue(math.Abs(a.Number)), nil
			}
			return intValue(math.Abs(a.Number)), nil
		})
	case "floor":
		return evalUnaryMath(n, ctx, func(a Value) (Value, error) { return roundedInt(a, math.Floor), nil })
	case "ceil":
		return evalUnaryMath(n, ctx, func(a Value) (Value, error) { return roundedInt(a, math.Ceil), nil })
	case "sin":
		return evalUnaryMath(n, ctx, floatFn(math.Sin))
	case "cos":
		return evalUnaryMath(n, ctx, floatFn(math.Cos))
	case "tan":
		return evalUnaryMath(n, ctx, floatFn(math.Tan))
	case "asin":
		return evalUnaryMath(n, ctx, floatFn(math.Asin))
	case "acos":
		return evalUnaryMath(n, ctx, floatFn(math.Acos))
	case "atan":
		return evalUnaryMath(n, ctx, floatFn(math.Atan))
	case "exp":
		return evalUnaryMath(n, ctx, floatFn(math.Exp))
	case "ln":
		return evalUnaryMath(n, ctx, func(a Value) (Value, error) {
			if a.Number <= 0 {
				return Value{}, fmt.Errorf("evaluator: Ln(%v): invalid logarithm", a.Number)
			}
			return floatValue(float64(float32(math.Log(a.Number)))), nil
		})
	case "log":
		return evalLog(n, ctx)
	}
	return Value{}, false, nil
}

// defaultLocalCoordWidth is the character's localcoord width Const240p and its
// siblings scale from: Ikemen GO's c.stOgi().localcoord[0] (char.go:5746).
// shortcut: taken as the 320x240 default, the .def's [Info] localcoord is not
// read yet.
const defaultLocalCoordWidth = 320

// evalConstScale is Const240p(v) and its siblings: v scaled from a screen of
// the given width to the character's own coordinates, in float32 as
// Ikemen GO's constp does (char.go:5746, bytecode.go:3432-3439).
func evalConstScale(n callNode, ctx *Context, coordinate float32) (Value, bool, error) {
	if len(n.args) != 1 {
		return Value{}, true, fmt.Errorf("evaluator: %s() expects 1 argument, got %d", n.name, len(n.args))
	}
	a, err := evalNode(n.args[0], ctx)
	if err != nil {
		return Value{}, true, err
	}
	return floatValue(float64(defaultLocalCoordWidth / coordinate * float32(a.Number))), true, nil
}

// floatFn lifts a float64 function to a Value function with Ikemen GO's
// float32 result (BytecodeExp.sin and friends call SetF(float32(...))).
func floatFn(f func(float64) float64) func(Value) (Value, error) {
	return func(a Value) (Value, error) { return floatValue(float64(float32(f(a.Number)))), nil }
}

// roundedInt is Floor/Ceil: a float rounds to an int, an int stays.
func roundedInt(a Value, round func(float64) float64) Value {
	if a.Kind == KindFloat {
		return intValue(round(a.Number))
	}
	return a
}

func evalUnaryMath(n callNode, ctx *Context, f func(Value) (Value, error)) (Value, bool, error) {
	if len(n.args) != 1 {
		return Value{}, true, fmt.Errorf("evaluator: %s() expects 1 argument, got %d", n.name, len(n.args))
	}
	a, err := evalNode(n.args[0], ctx)
	if err != nil {
		return Value{}, true, err
	}
	v, err := f(a)
	return v, true, err
}

// evalLog is Log(a, b): the logarithm of b in base a (BytecodeExp.log).
func evalLog(n callNode, ctx *Context) (Value, bool, error) {
	if len(n.args) != 2 {
		return Value{}, true, fmt.Errorf("evaluator: Log() expects 2 arguments, got %d", len(n.args))
	}
	base, err := evalNode(n.args[0], ctx)
	if err != nil {
		return Value{}, true, err
	}
	arg, err := evalNode(n.args[1], ctx)
	if err != nil {
		return Value{}, true, err
	}
	if base.Number <= 0 || arg.Number <= 0 {
		return Value{}, true, fmt.Errorf("evaluator: Log(%v, %v): invalid logarithm", base.Number, arg.Number)
	}
	return floatValue(float64(float32(math.Log(arg.Number) / math.Log(base.Number)))), true, nil
}

// typeComparison recognizes "StateType = S" and "MoveType != H": a state or
// move type trigger compared to a bare letter. It returns the trigger name,
// the letter upper-cased, and true only for exactly that form.
func typeComparison(n binaryNode) (trigger string, letter byte, ok bool) {
	if n.op != "=" && n.op != "!=" {
		return "", 0, false
	}
	id, ok := n.left.(identifierNode)
	if !ok || (id.name != "statetype" && id.name != "movetype" && id.name != "p2statetype" && id.name != "p2movetype") {
		return "", 0, false
	}
	l, ok := n.right.(identifierNode)
	if !ok || len(l.name) != 1 {
		return "", 0, false
	}
	letter = l.name[0]
	if letter >= 'a' && letter <= 'z' {
		letter -= 'a' - 'A'
	}
	return id.name, letter, true
}

func evalTypeComparison(n binaryNode, ctx *Context) (Value, bool, error) {
	trigger, letter, ok := typeComparison(n)
	if !ok {
		return Value{}, false, nil
	}
	valid, current := "SCAL", ctx.StateType
	switch trigger {
	case "movetype":
		valid, current = "IAH", ctx.MoveType
	case "p2statetype":
		current = ctx.P2.StateType
	case "p2movetype":
		valid, current = "IAH", ctx.P2.MoveType
	}
	if strings.IndexByte(valid, letter) < 0 {
		return Value{}, true, fmt.Errorf("evaluator: %s cannot be compared to %q (expected one of %s)", trigger, string(letter), valid)
	}
	same := current == letter
	if n.op == "!=" {
		same = !same
	}
	return boolValue(same), true, nil
}
