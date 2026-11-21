package evaluator

import (
	"fmt"
	"math"
	"strings"

	"github.com/openkakutou/engine/v2/match"
)

// axisTriggers are the triggers written with an axis letter ("Pos Y").
var axisTriggers = map[string]bool{"pos": true, "vel": true, "p2dist": true, "p2bodydist": true, "camerapos": true,
	"parentdist": true, "rootdist": true, "screenpos": true}

// firstPlayerID is player 1's ID, Ikemen GO's default HelperMax.
const firstPlayerID = 56

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
		// sys.roundState (system.go:1908-1923), set by the engine each tick.
		return intValue(float64(ctx.RoundState)), true, nil
	case "backedgedist":
		return floatValue(edgeDist(ctx, false)), true, nil // char.go:5641-5646
	case "frontedgedist":
		return floatValue(edgeDist(ctx, true)), true, nil // char.go:5779-5784
	case "backedgebodydist":
		// char.go:5627-5639; widthEdge is the Width controller's.
		return floatValue(edgeDist(ctx, false) - ctx.WidthEdge[1] - mugenBodyOffset(ctx)), true, nil
	case "frontedgebodydist":
		return floatValue(edgeDist(ctx, true) - ctx.WidthEdge[0] - mugenBodyOffset(ctx)), true, nil // char.go:5766-5777
	case "inguarddist":
		return boolValue(inGuardDist(ctx)), true, nil
	case "gametime":
		// sys.gameTime (system.go:1892-1903): the match's ticks plus the
		// time before it. shortcut: it is the tick count the caller gives
		// (TickConfig.Tick); the time before the match is not counted.
		return intValue(float64(ctx.GameTime)), true, nil
	case "pi":
		// compiler.go:4383-4386: float32 constants.
		return floatValue(float64(float32(math.Pi))), true, nil
	case "e":
		return floatValue(float64(float32(math.E))), true, nil
	case "roundsexisted":
		// sys.roundsExisted (char.go:6338-6343): the rounds finished, the
		// round number less one for a one-on-one fighter.
		return intValue(float64(max(ctx.Round-1, 0))), true, nil
	case "ishometeam":
		// teamside == sys.home, 0 by default (bytecode.go:3428-3429).
		return boolValue(ctx.Side == 0), true, nil
	case "win", "winko", "wintime", "winperfect", "lose", "loseko", "losetime":
		// char.go:5952-5957, 6597-6610: the decided round's winner and how
		// it was won.
		return boolValue(roundEndTrigger(name, ctx)), true, nil
	case "drawgame":
		return boolValue(ctx.RoundEnd.Decided && ctx.RoundEnd.Draw), true, nil
	case "matchover":
		return boolValue(ctx.RoundEnd.MatchOver), true, nil
	case "winspecial", "winhyper", "winclutch":
		// shortcut: the finishing move's kind is not tracked.
		return boolValue(false), true, nil
	case "matchno":
		return intValue(1), true, nil
	case "lifemax":
		return intValue(float64(constOr(ctx, "data.life", 1000))), true, nil
	case "powermax":
		return intValue(float64(constOr(ctx, "data.power", 3000))), true, nil
	case "camerazoom":
		return floatValue(1), true, nil
	case "screenwidth":
		// char.go:6381-6383: the character's localcoord width.
		// shortcut: 320, the MUGEN default; a .def localcoord is not read.
		return intValue(320), true, nil
	case "screenheight":
		// char.go:6367-6379: the width over the fight's aspect. shortcut: 4:3.
		return intValue(240), true, nil
	case "gamewidth":
		// shortcut: a 320 by 240 game, the size MUGEN characters are written
		// for (localcoord 320).
		return intValue(320), true, nil
	case "gameheight":
		return intValue(240), true, nil
	case "hitcount", "uniqhitcount":
		// char.go:13125-13129 (uniqhitcount counts once per target; one target
		// here).
		return intValue(float64(ctx.HitCount)), true, nil
	case "teamside":
		// OC_teamside (bytecode.go:2735-2736): the side's team, 1 or 2; a
		// helper is on its player's.
		return intValue(float64(int(ctx.Side) + 1)), true, nil
	case "numpartner":
		// bytecode.go OC_numpartner: a one-on-one match has no partner.
		return intValue(0), true, nil
	case "numproj":
		// char.go:6186-6191 counts live projectiles.
		return intValue(float64(ctx.NumProjectiles)), true, nil
	case "hitpausetime":
		return intValue(float64(ctx.HitPauseTime)), true, nil // bytecode.go:2615-2616
	case "id":
		// The players' IDs start at HelperMax, 56 (system.go:2277-2325).
		return intValue(float64(firstPlayerID + int(ctx.Side))), true, nil
	case "standby":
		// bytecode.go:3936-3937 reads SCF_standby, which only Tag mode sets
		// (bytecode.go:5888-5894); a one-on-one fighter is never on standby.
		return boolValue(false), true, nil
	case "ishelper":
		return boolValue(ctx.IsHelper), true, nil
	case "random":
		return intValue(float64(drawRandom(ctx))), true, nil
	case "ailevel":
		// Ikemen GO reads sys.aiLevel[playerNo] truncated (bytecode.go:2510-2515,
		// char.go:5529-5537), 0 for a human player or under noailevel.
		if ctx.Asserted.Has("noailevel") {
			return intValue(0), true, nil
		}
		return intValue(math.Trunc(ctx.AILevel)), true, nil
	case "ailevelf":
		if ctx.Asserted.Has("noailevel") {
			return floatValue(0), true, nil
		}
		return floatValue(ctx.AILevel), true, nil // bytecode.go:3654-3659
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
		// char.go:5975-5980: the contact time of a contact that hit.
		if ctx.ContactGuarded {
			return intValue(0), true, nil
		}
		return intValue(float64(ctx.MoveContactTime)), true, nil
	case "moveguarded":
		// char.go:5981-5986: the contact time of a guarded contact.
		if !ctx.ContactGuarded {
			return intValue(0), true, nil
		}
		return intValue(float64(ctx.MoveContactTime)), true, nil
	case "movereversed":
		// char.go:5993-5999: ticks since its move was reversed.
		return intValue(float64(ctx.MoveReversedTime)), true, nil
	case "hitfall":
		return boolValue(ctx.GetHit.Fall), true, nil
	case "canrecover":
		return boolValue(ctx.GetHit.FallRecover && ctx.GetHit.FallTime >= ctx.GetHit.FallRecoverTime), true, nil
	case "numhelper":
		// bytecode.go:2657-2660 counts the player's helpers.
		return intValue(float64(len(ctx.Helpers))), true, nil
	case "numexplod":
		// The fighter's own explods (char.go:7326-7350 getMultipleExplods).
		return intValue(float64(len(ctx.ExplodIDs))), true, nil
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
	return nextRandom(ctx) / (randomMax/1000 + 1)
}

// RandomRange is Ikemen GO's RandI(lo, hi) (common.go:43-62) on the match's
// shared seed: an integer from lo to hi, either order.
func RandomRange(ctx *Context, lo, hi int32) int32 {
	if hi < lo {
		lo, hi = hi, lo
	}
	if uint32(hi-lo) > uint32(randomMax) {
		return int32(int64(lo) + int64(nextRandom(ctx))*(int64(hi)-int64(lo))/int64(randomMax))
	}
	return lo + nextRandom(ctx)/(randomMax/(hi-lo+1)+1)
}

// nextRandom advances ctx.RandSeed one step of Ikemen GO's Random.
func nextRandom(ctx *Context) int32 {
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
	return seed
}

// evalComponent evaluates "Pos X/Y/Z", "Vel X/Y/Z", "P2Dist X/Y/Z",
// "P2BodyDist X/Y/Z" and "CameraPos X/Y".
func evalComponent(n componentNode, ctx *Context) (Value, error) {
	var x, y float64
	switch n.name {
	case "p2dist", "p2bodydist":
		return evalP2Distance(n, ctx)
	case "pos":
		x, y = ctx.Position.X, ctx.Position.Y
	case "parentdist", "rootdist":
		// The parent's or root's distance, x toward the fighter's facing, y
		// down (char.go parentDist/rootDist); 0 without one.
		other := ctx.Parent
		if n.name == "rootdist" {
			other = ctx.Root
		}
		if other.Valid {
			x = (other.Position.X - ctx.Position.X) * facingSign(ctx.Facing)
			y = other.Position.Y - ctx.Position.Y
		}
	case "screenpos":
		// shortcut: from the screen's left edge less its margin and from a top
		// edge 220 above the ground (a 240-high screen), the camera fixed in
		// height.
		x = ctx.Position.X - ctx.ScreenLeft
		y = ctx.Position.Y - 220
	case "camerapos":
		// sys.cam.Pos (bytecode.go:2536-2539): the camera's centre,
		// halfway between the screen edges; the camera does not move up.
		x = (ctx.ScreenLeft + ctx.ScreenRight) / 2
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
		// isHelper (char.go:5841-5864): a helper, and with an id that helper
		// id. shortcut: the second, index argument is not read.
		if len(n.args) == 0 {
			return boolValue(ctx.IsHelper), true, nil
		}
		id, err := evalNode(n.args[0], ctx)
		if err != nil {
			return Value{}, true, err
		}
		if id.Int() < 0 {
			return boolValue(ctx.IsHelper), true, nil
		}
		return boolValue(ctx.IsHelper && ctx.HelperID == id.Int()), true, nil
	case "numhelper":
		// numHelper (char.go:6162-6177): the player's helpers with that id, all
		// of them for an id of 0 or less.
		if len(n.args) > 1 {
			return Value{}, true, fmt.Errorf("evaluator: numhelper() expects at most 1 argument, got %d", len(n.args))
		}
		id := 0
		for _, arg := range n.args {
			v, err := evalNode(arg, ctx)
			if err != nil {
				return Value{}, true, err
			}
			id = v.Int()
		}
		count := 0
		for _, h := range ctx.Helpers {
			if id <= 0 || h.HelperID == id {
				count++
			}
		}
		return intValue(float64(count)), true, nil
	case "playeridexist":
		// The players' ids (see "id"). shortcut: helpers have no id of their
		// own yet, so only the players exist.
		if len(n.args) != 1 {
			return Value{}, true, fmt.Errorf("evaluator: playeridexist() expects 1 argument, got %d", len(n.args))
		}
		id, err := evalNode(n.args[0], ctx)
		if err != nil {
			return Value{}, true, err
		}
		return boolValue(id.Int() == firstPlayerID || id.Int() == firstPlayerID+1), true, nil
	case "stagevar":
		// compiler.go:4044-4100. shortcut: the engine does not know the
		// stage's info or bounds here: a number reads 0 and a name "" (see
		// stageVarComparison).
		return intValue(0), true, nil
	case "numprojid":
		if len(n.args) != 1 {
			return Value{}, true, fmt.Errorf("evaluator: numprojid() expects 1 argument, got %d", len(n.args))
		}
		id, err := evalNode(n.args[0], ctx)
		if err != nil {
			return Value{}, true, err
		}
		count := 0
		for _, p := range ctx.ProjectileIDs {
			if p == id.Int() {
				count++
			}
		}
		return intValue(float64(count)), true, nil
	case "projhittime", "projguardedtime", "projcontacttime", "projcanceltime":
		if len(n.args) != 1 {
			return Value{}, true, fmt.Errorf("evaluator: %s() expects 1 argument, got %d", n.name, len(n.args))
		}
		id, err := evalNode(n.args[0], ctx)
		if err != nil {
			return Value{}, true, err
		}
		kind := strings.TrimSuffix(strings.TrimPrefix(n.name, "proj"), "time")
		return intValue(float64(projTime(ctx, kind, id.Int()))), true, nil
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
	case "numexplod":
		// The optional id picks which of the fighter's explods to count, all
		// for a negative one (compiler.go:3415-3425, Explod.matchId).
		if len(n.args) > 1 {
			return Value{}, true, fmt.Errorf("evaluator: numexplod() expects at most 1 argument, got %d", len(n.args))
		}
		id := -1
		for _, arg := range n.args {
			v, err := evalNode(arg, ctx)
			if err != nil {
				return Value{}, true, err
			}
			id = v.Int()
		}
		count := 0
		for _, e := range ctx.ExplodIDs {
			if id < 0 || e == id {
				count++
			}
		}
		return intValue(float64(count)), true, nil
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

// edgeDist is the distance from the fighter to the screen edge in front of
// it (front) or behind it.
func edgeDist(ctx *Context, front bool) float64 {
	right := ctx.Facing == match.FacingRight
	if front == right {
		return ctx.ScreenRight - ctx.Position.X
	}
	return ctx.Position.X - ctx.ScreenLeft
}

// InGuardDist is the InGuardDist trigger, for the engine's own guard entry.
func InGuardDist(ctx *Context) bool { return inGuardDist(ctx) }

// inGuardDist is Ikemen GO's guard distance check (char.go:14159-14183) seen
// from the defender: the opponent attacks with an active HitDef whose
// guard.dist reaches it.
// shortcut: the y and z guard distances are not checked; their defaults
// reach everything a 2D match has.
func inGuardDist(ctx *Context) bool {
	o := ctx.P2
	if !o.Valid || o.MoveType != 'A' || o.HitDefAttr == 0 {
		return false
	}
	dist := ctx.Position.X - o.Position.X
	if o.Facing == match.FacingLeft {
		dist = -dist
	}
	return dist < o.GuardDist[0] && dist > -o.GuardDist[1]
}

// mugenBodyOffset is how much nearer an edge a MUGEN character's body is in
// the air (0.5) or lying down (1), an undocumented MUGEN quirk Ikemen GO
// keeps for characters without an ikemenversion (char.go:5628-5637).
func mugenBodyOffset(ctx *Context) float64 {
	if ctx.IkemenChar {
		return 0
	}
	switch ctx.StateType {
	case 'A':
		return 0.5
	case 'L':
		return 1
	}
	return 0
}

// constOr is a [Data] constant, def when the character has none.
func constOr(ctx *Context, name string, def int) int {
	if ctx.Consts == nil {
		return def
	}
	v := ctx.Consts.Lookup(name)
	if v.Int() == 0 {
		return def
	}
	return v.Int()
}

// roundEndTrigger is Win, WinKO, WinTime, WinPerfect, Lose, LoseKO or
// LoseTime on the decided round (char.go:5952-5957, 6597-6620).
func roundEndTrigger(name string, ctx *Context) bool {
	end := ctx.RoundEnd
	if !end.Decided || end.Draw {
		return false
	}
	won := end.Winner == ctx.Side
	if strings.HasPrefix(name, "lose") {
		won = !won
	}
	switch name {
	case "winko", "loseko":
		return won && end.KO
	case "wintime", "losetime":
		return won && !end.KO
	case "winperfect":
		return won && ctx.Health >= constOr(ctx, "data.life", 1000)
	}
	return won
}
