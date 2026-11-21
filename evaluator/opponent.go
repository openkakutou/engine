package evaluator

import (
	"fmt"
	"github.com/openkakutou/character/air"
	"math"
	"strings"

	"github.com/openkakutou/engine/v2/match"
)

// Opponent is what a fighter's triggers can read of the opponent: a snapshot
// Tick takes before the fighter acts, copied into Context.P2. It is a value,
// not a pointer to the other fighter's Context: Context travels through
// indirect calls (the controller registry) that make anything it points to
// escape to the heap, which would cost an allocation per fighter per tick
// (decisions 009, 016).
//
// It is also what a redirection (p2, enemy, parent, root, helper(id)...)
// reads of the fighter it names.
type Opponent struct {
	// Valid reports that there is an opponent: false on the zero value, where
	// every opponent trigger reads 0, as Ikemen GO's undefined value does for a
	// nil p2() (char.go:9318-9321).
	Valid bool
	match.FighterState
	Time, Anim, PrevStateNo int
	Ctrl                    bool
	StateType, MoveType     byte
	GetHit                  GetHit
	MoveContactTime         int
	HitDefAttr              int32
	// GuardDist is the opponent's HitDef guard.dist x reach, front and back.
	GuardDist [2]float64
	// ScreenLeft, ScreenRight, WidthEdge and IkemenChar are what its edge
	// distances read.
	ScreenLeft, ScreenRight float64
	WidthEdge               [2]float64
	IkemenChar              bool
	NumProjectiles          int
	// Name and AuthorName are its character's, for P2Name and the
	// redirected name triggers.
	Name, AuthorName string
	// HitPauseTime, AnimElapsed, Anims and PalNo are what HitPauseTime,
	// AnimTime, AnimElemTime(n), AnimExist(n) and PalNo read.
	HitPauseTime, AnimElapsed, PalNo int
	Anims                            map[int]air.Animation
	// Helpers are its player's helpers, for a redirected NumHelper.
	Helpers []Opponent
	// IsHelper and HelperID tell a helper (parent, root and helper(id)
	// redirections read them).
	IsHelper bool
	HelperID int
	// Vars, FVars and SysVars are what var(n), fvar(n) and sysvar(n) read
	// through a redirection (root,var(5) in a helper's states).
	Vars    [maxVars]int
	FVars   [maxFVars]float32
	SysVars [maxSysVars]int
	// Consts is the opponent's constants, for its size box and Const(...).
	Consts *Constants
}

// context is the opponent as the Context an expression redirected to it
// reads. Fields the snapshot does not carry are zero.
func (o *Opponent) context() Context {
	return Context{
		FighterState:    o.FighterState,
		Time:            o.Time,
		Anim:            o.Anim,
		PrevStateNo:     o.PrevStateNo,
		Ctrl:            o.Ctrl,
		StateType:       o.StateType,
		MoveType:        o.MoveType,
		GetHit:          o.GetHit,
		MoveContactTime: o.MoveContactTime,
		HitDefAttr:      o.HitDefAttr,
		Consts:          o.Consts,
		ScreenLeft:      o.ScreenLeft,
		ScreenRight:     o.ScreenRight,
		WidthEdge:       o.WidthEdge,
		IkemenChar:      o.IkemenChar,
		NumProjectiles:  o.NumProjectiles,
		Name:            o.Name,
		AuthorName:      o.AuthorName,
		IsHelper:        o.IsHelper,
		HelperID:        o.HelperID,
		Helpers:         o.Helpers,
		HitPauseTime:    o.HitPauseTime,
		AnimElapsed:     o.AnimElapsed,
		Anims:           o.Anims,
		PalNo:           o.PalNo,
		Vars:            o.Vars,
		FVars:           o.FVars,
		SysVars:         o.SysVars,
	}
}

// redirectKinds are the redirection keywords the parser knows, true for those
// the engine answers. The others parse and then fail with "not supported yet":
// there are no helpers yet (backlog item 029), so a redirection to one cannot
// be answered, and reading 0 would be wrong.
var redirectKinds = map[string]bool{
	"p2": true, "enemy": true, "enemynear": true, "target": true,
	"root": true, "parent": true, "helper": true, "playerid": false,
	"player": false, "partner": true, "stateowner": false,
}

// IsRedirect reports whether word (any case) is a redirection keyword, which
// takes the comma after it (and after its "(index)") as its own.
func IsRedirect(word string) bool {
	_, ok := redirectKinds[strings.ToLower(word)]
	return ok
}

// redirectNode is "p2, trigger", "enemy(n), trigger", "enemyNear(n), trigger"
// or "target(id), trigger": the inner trigger read on another fighter. kind is
// lower-case, arg the optional index (nil when absent).
type redirectNode struct {
	kind  string
	arg   node
	inner node
}

func (redirectNode) isNode() {}

// hitDefAttrNode is "HitDefAttr = attr" or "HitDefAttr != attr".
type hitDefAttrNode struct {
	attr int32
	not  bool
}

func (hitDefAttrNode) isNode() {}

// evalRedirect reads n.inner on the fighter the redirection names. A
// redirection to a fighter that does not exist reads 0 (Ikemen GO's nil char,
// whose triggers are undefined, bytecode.go:2298-2308).
//
// Any trigger is read on the redirected fighter's snapshot: one the snapshot
// does not carry (its commands, its random draws) reads as a fighter with
// none would, rather than stopping the match.
func evalRedirect(n redirectNode, ctx *Context) (Value, error) {
	target, found, err := redirectTarget(n, ctx)
	if err != nil || !found {
		return intValue(0), err
	}
	redirected := target.context()
	// The match's own state reads the same from every fighter.
	redirected.Round, redirected.GameTime = ctx.Round, ctx.GameTime
	return evalNode(n.inner, &redirected)
}

// redirectTarget is the Opponent a redirection reads, found false when there is
// none. With one opponent, p2 and enemy(0) and enemyNear(0) are it, enemy(n)
// and enemyNear(n) for n > 0 do not exist (char.go:5428-5462), and target(id)
// is it while it is a target (see isTarget). It returns the snapshot by value:
// a pointer into ctx would make ctx escape to the heap.
func redirectTarget(n redirectNode, ctx *Context) (target Opponent, found bool, err error) {
	index := 0
	if n.arg != nil {
		v, err := evalNode(n.arg, ctx)
		if err != nil {
			return Opponent{}, false, err
		}
		index = v.Int()
	}
	switch n.kind {
	case "p2":
		found = ctx.P2.Valid
	case "enemy", "enemynear":
		found = ctx.P2.Valid && index == 0
	case "target":
		if n.arg == nil {
			index = -1
		}
		found = isTarget(ctx, index)
	case "partner":
		// A one-on-one fighter has no partner (char.go partner).
		return Opponent{}, false, nil
	case "parent":
		return ctx.Parent, ctx.Parent.Valid, nil
	case "root":
		return ctx.Root, ctx.Root.Valid, nil
	case "helper":
		// helper(id) is the first of the fighter's helpers with that id, helper
		// the first of them (char.go:5383-5410).
		for _, h := range ctx.Helpers {
			if n.arg == nil || h.HelperID == index {
				return h, true, nil
			}
		}
		return Opponent{}, false, nil
	}
	return ctx.P2, found, nil
}

// isTarget reports that the opponent is the fighter's target: it has hit it and
// the opponent is still in a hit state, the only way Ikemen GO keeps a target
// (char.go:10387-10404); and, when id is not negative, that its last hit carried
// that HitDef id (char.go:6211-6224).
func isTarget(ctx *Context, id int) bool {
	if !ctx.HasTarget || !ctx.P2.Valid || ctx.P2.MoveType != 'H' {
		return false
	}
	return id < 0 || ctx.P2.GetHit.HitID == id
}

// facingSign is the multiplier of a facing-relative X: 1 facing right, -1 left.
func facingSign(f match.Facing) float64 {
	if f == match.FacingRight {
		return 1
	}
	return -1
}

// distSnap is Ikemen GO's snap of a tiny X distance to 0 (char.go:9217).
const distSnap = 0.0001

// distX is Char.distX (char.go:9200-9221): the opponent's X minus the
// fighter's, snapped to 0 when negligible.
func distX(ctx *Context) float64 {
	d := ctx.P2.Position.X - ctx.Position.X
	if math.Abs(d) < distSnap {
		return 0
	}
	return d
}

// distY is Char.distY (char.go:9223): the content's Y points down and the
// engine's up, so each Position.Y is negated.
func distY(ctx *Context) float64 {
	return -ctx.P2.Position.Y + ctx.Position.Y
}

// sizeFrontBack is a fighter's size box edges, front and back as positive
// widths: the air box in state type A, the stand box otherwise (Ikemen GO's
// baseSizeBox, char.go:10484-10495).
//
// shortcut: the crouch and down boxes (crouch.sizebox, down.sizebox) are not
// read by Constants, so C and L use the stand box, and Width controllers do
// not widen it. Ikemen GO's defaults give all of C and L the stand box too.
func sizeFrontBack(consts *Constants, stateType byte) (front, back float64) {
	if stateType == 'A' {
		return consts.Lookup("size.air.front").Number, consts.Lookup("size.air.back").Number
	}
	return consts.Lookup("size.ground.front").Number, consts.Lookup("size.ground.back").Number
}

// bodyDistX is Char.p2BodyDistX over bodyDistX (char.go:9244-9276, 9354-9363),
// in the fighter's facing: the gap between the two size boxes. The fighter's
// reference edge is always its front, the opponent's is its front or back
// depending on which side of the fighter it stands and which way it faces.
func bodyDistX(ctx *Context) float64 {
	dist := distX(ctx)
	facing, oppFacing := facingSign(ctx.Facing), facingSign(ctx.P2.Facing)
	front, _ := sizeFrontBack(ctx.Consts, ctx.StateType)
	oppFront, oppBack := sizeFrontBack(ctx.P2.Consts, ctx.P2.StateType)
	var oppEdge float64
	if (dist*facing >= 0) == (facing != oppFacing) {
		oppEdge = oppFront * oppFacing
	} else {
		oppEdge = -oppBack * oppFacing
	}
	return facing * (dist - front*facing + oppEdge)
}

// evalP2Distance evaluates "P2Dist X/Y" and "P2BodyDist X/Y". P2BodyDist Y is
// P2Dist Y: Ikemen GO answers with the size boxes' vertical gap only for an
// Ikemen GO character (char.go:9366-9372), and a character's version is not
// read here; kfm and every MUGEN character take the P2Dist Y branch.
// shortcut: Ikemen GO truncates P2Dist X/Y and P2BodyDist X to an integer
// for a WinMUGEN character (mugenversion major != 1, char.go:9324-9361);
// character does not expose mugenversion yet. Upgrade when it does.
func evalP2Distance(n componentNode, ctx *Context) (Value, error) {
	if n.axis == 'z' {
		return Value{}, fmt.Errorf("evaluator: %s Z is not supported: fighters have no depth", n.name)
	}
	if !ctx.P2.Valid {
		return floatValue(0), nil
	}
	switch {
	case n.axis == 'y':
		return floatValue(distY(ctx)), nil
	case n.name == "p2bodydist":
		return floatValue(bodyDistX(ctx)), nil
	}
	return floatValue(facingSign(ctx.Facing) * distX(ctx)), nil
}
