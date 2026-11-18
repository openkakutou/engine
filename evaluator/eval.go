// Package evaluator parses and evaluates MUGEN CNS trigger/parameter
// expressions — the strings character/cns's Controller deliberately leaves
// unevaluated (Controller.Triggers []string, Controller.Parameters
// map[string]string; see that package's decision 011). It supports
// comparisons (=, !=, <, >, <=, >=), boolean/arithmetic operators (&&, ||,
// +, -, *, /, %, unary !), and a growing set of built-in trigger names and
// functions: var(), sysvar(), IfElse(), Time, StateNo, Anim, AnimTime,
// Command, Ctrl, HitShakeOver, HitOver and GetHitVar(). Later engine items are expected to extend the
// built-in set incrementally as they need more of MUGEN's trigger
// vocabulary — this package deliberately does not attempt to implement it
// exhaustively up front.
package evaluator

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// Kind classifies a Value the same way MUGEN's own loosely-typed triggers
// do: a trigger result is fundamentally always a number, but callers often
// care whether it originated as a boolean, integer, or float expression.
type Kind int

const (
	// KindBool is the result of a comparison or logical operator.
	KindBool Kind = iota
	// KindInt is the result of an integer literal or arithmetic between
	// two integer-kinded operands.
	KindInt
	// KindFloat is the result of a float literal, or arithmetic where
	// either operand was float-kinded.
	KindFloat
)

// Value is a MUGEN-style trigger result. Every Kind stores its value in
// Number (a boolean as 1 or 0); Bool, Float, and Int expose it as whichever
// Go type the caller needs, matching MUGEN's own loose typing rules (e.g.
// true = 1, 3 = 3.0 both compare equal).
type Value struct {
	Kind   Kind
	Number float64
}

// Bool reports Number != 0 — MUGEN's rule for treating any numeric value
// as a boolean.
func (v Value) Bool() bool { return v.Number != 0 }

// Float returns Number as a float64.
func (v Value) Float() float64 { return v.Number }

// Int returns Number truncated to an int.
func (v Value) Int() int { return int(v.Number) }

func boolValue(b bool) Value {
	if b {
		return Value{Kind: KindBool, Number: 1}
	}
	return Value{Kind: KindBool, Number: 0}
}

func intValue(n float64) Value   { return Value{Kind: KindInt, Number: n} }
func floatValue(n float64) Value { return Value{Kind: KindFloat, Number: n} }

// parseCache memoizes Parse by source string, so Evaluate's repeated calls
// with the same expr (e.g. a controller's trigger checked every simulation
// tick, across statemachine.Step/zssexec.Step calls) reuse the already-
// built Expression instead of re-lexing/re-parsing it from scratch each
// time -- CNS/`.zss` trigger and parameter strings are loaded once per
// character and never change for the life of a match, so the set of
// distinct keys stays bounded by the loaded character data, not by tick
// count. Safe as a plain, unsynchronized map: this engine targets a
// single-threaded GOOS=js GOARCH=wasm build with no goroutines anywhere in
// the simulation loop (see .vibe/decisions/009-011).
var parseCache = make(map[string]*Expression)

// Evaluate is a convenience that parses expr (reusing a cached parse if
// expr has been seen before -- see parseCache) and evaluates it against
// ctx in one call. Prefer Parse followed by repeated Expression.Eval calls
// when the caller already holds on to the same *Expression across calls;
// Evaluate itself now also avoids the repeated-parse cost via parseCache.
func Evaluate(expr string, ctx Context) (Value, error) {
	e, err := cachedParse(expr)
	if err != nil {
		return Value{}, err
	}
	return e.Eval(ctx)
}

// EvaluateMut is Evaluate through EvalMut: it may advance ctx.RandSeed. A
// ctx with ZSS set evaluates as EvaluateMutShortCircuit does.
func EvaluateMut(expr string, ctx *Context) (Value, error) {
	if ctx.ZSS {
		return EvaluateMutShortCircuit(expr, ctx)
	}
	e, err := cachedParse(expr)
	if err != nil {
		return Value{}, err
	}
	return e.EvalMut(ctx)
}

// shortCircuitParseCache is parseCache for ParseShortCircuit.
var shortCircuitParseCache = make(map[string]*Expression)

// EvaluateMutShortCircuit is EvaluateMut for a .zss expression, parsed by
// ParseShortCircuit.
func EvaluateMutShortCircuit(expr string, ctx *Context) (Value, error) {
	e, ok := shortCircuitParseCache[expr]
	if !ok {
		var err error
		if e, err = ParseShortCircuit(expr); err != nil {
			return Value{}, err
		}
		shortCircuitParseCache[expr] = e
	}
	return e.EvalMut(ctx)
}

// cachedParse is Parse behind parseCache.
func cachedParse(expr string) (*Expression, error) {
	e, ok := parseCache[expr]
	if ok {
		return e, nil
	}
	e, err := Parse(expr)
	if err != nil {
		return nil, err
	}
	parseCache[expr] = e
	return e, nil
}

// UnknownTriggerError reports a trigger identifier (Call false) or
// function-style trigger (Call true) the evaluator does not implement. Name
// is lower-cased, as the evaluator matches it.
type UnknownTriggerError struct {
	Name string
	Call bool
}

func (e *UnknownTriggerError) Error() string {
	if e.Call {
		return fmt.Sprintf("evaluator: unknown trigger function %q", e.Name)
	}
	return fmt.Sprintf("evaluator: unknown trigger %q", e.Name)
}

// TriggerName is one trigger identifier or function name used by an
// expression, lower-cased.
type TriggerName struct {
	Name string
	Call bool
}

// TriggerNames returns every trigger identifier and function name expr
// uses, in first-appearance order without duplicates. Quoted strings and
// the GetHitVar argument (a variable name read as written) are not
// trigger names. A parse error is returned as is.
func TriggerNames(expr string) ([]TriggerName, error) {
	e, err := cachedParse(expr)
	if err != nil {
		return nil, err
	}
	var names []TriggerName
	seen := make(map[TriggerName]bool)
	collectTriggerNames(e.root, &names, seen)
	return names, nil
}

func collectTriggerNames(n node, names *[]TriggerName, seen map[TriggerName]bool) {
	add := func(t TriggerName) {
		if !seen[t] {
			seen[t] = true
			*names = append(*names, t)
		}
	}
	switch tn := n.(type) {
	case identifierNode:
		add(TriggerName{Name: tn.name})
	case callNode:
		add(TriggerName{Name: tn.name, Call: true})
		if tn.name == "const" {
			return // its argument is a constant name, not a trigger
		}
		if tn.name == "gethitvar" {
			// One entry per variable, named "gethitvar(<var>)", so a variable
			// evalGetHitVar does not handle can be reported by IsKnownTrigger.
			if len(tn.args) == 1 {
				if id, ok := tn.args[0].(identifierNode); ok {
					add(TriggerName{Name: "gethitvar(" + id.name + ")"})
				}
			}
			return
		}
		for _, arg := range tn.args {
			collectTriggerNames(arg, names, seen)
		}
	case animElemNode:
		add(TriggerName{Name: "animelem"})
		collectTriggerNames(tn.value, names, seen)
	case redirectNode:
		if tn.arg != nil {
			collectTriggerNames(tn.arg, names, seen)
		}
		collectTriggerNames(tn.inner, names, seen)
	case hitDefAttrNode:
		add(TriggerName{Name: "hitdefattr"})
	case unaryNode:
		collectTriggerNames(tn.operand, names, seen)
	case assignNode:
		collectTriggerNames(tn.index, names, seen)
		collectTriggerNames(tn.value, names, seen)
	case componentNode:
		add(TriggerName{Name: tn.name})
	case binaryNode:
		if trigger, _, ok := typeComparison(tn); ok {
			add(TriggerName{Name: trigger})
			return
		}
		collectTriggerNames(tn.left, names, seen)
		collectTriggerNames(tn.right, names, seen)
	}
}

// IsKnownTrigger reports whether the evaluator implements the trigger
// identifier (call false) or function (call true) called name, by probing
// evalIdentifier/evalCall: only an UnknownTriggerError means unknown.
//
// A "gethitvar(<var>)" name (as TriggerNames yields, call false) is probed
// through evalGetHitVar: known when it handles the variable.
func IsKnownTrigger(name string, call bool) bool {
	var err error
	probe := &Context{}
	if variable, ok := strings.CutPrefix(name, "gethitvar("); ok && !call {
		variable = strings.TrimSuffix(variable, ")")
		_, err = evalGetHitVar(callNode{name: "gethitvar", args: []node{identifierNode{name: variable}}}, probe)
		var unknown *unknownGetHitVarError
		return !errors.As(err, &unknown)
	}
	if call {
		_, err = evalCall(callNode{name: name}, probe)
	} else {
		_, err = evalIdentifier(name, probe)
	}
	var ute *UnknownTriggerError
	return !errors.As(err, &ute)
}

// Eval evaluates the parsed expression against ctx, returning MUGEN's
// loosely-typed result. It returns a descriptive error — never a panic —
// when the expression references an unsupported/unknown trigger name or
// function, uses a value in an unsupported way (e.g. a bare string
// literal), or hits a runtime error such as an out-of-range var() index.
func (e *Expression) Eval(ctx Context) (Value, error) {
	if e.hasAssign {
		return Value{}, errAssignNeedsMut
	}
	return evalNode(e.root, &ctx)
}

// EvalMut is Eval for a caller that keeps ctx: evaluating Random advances
// ctx.RandSeed, as Ikemen GO's shared generator advances on every draw, so
// the caller threads the seed on by passing the same ctx to the next call.
// Eval works on a copy and leaves the caller's seed untouched.
func (e *Expression) EvalMut(ctx *Context) (Value, error) {
	return evalNode(e.root, ctx)
}

func evalNode(n node, ctx *Context) (Value, error) {
	switch tn := n.(type) {
	case numberNode:
		if tn.isFloat {
			return floatValue(tn.value), nil
		}
		return intValue(tn.value), nil

	case stringNode:
		return Value{}, fmt.Errorf("evaluator: string literal %q cannot be evaluated outside of a Command comparison", tn.value)

	case identifierNode:
		return evalIdentifier(tn.name, ctx)

	case callNode:
		return evalCall(tn, ctx)

	case assignNode:
		return evalAssign(tn, ctx)

	case localNode:
		return evalLocal(tn.name, ctx)

	case animElemNode:
		return evalAnimElem(tn, ctx)

	case redirectNode:
		return evalRedirect(tn, ctx)

	case hitDefAttrNode:
		return evalHitDefAttr(tn, ctx), nil

	case componentNode:
		return evalComponent(tn, ctx)

	case unaryNode:
		return evalUnary(tn, ctx)

	case binaryNode:
		return evalBinary(tn, ctx)

	default:
		return Value{}, fmt.Errorf("evaluator: internal error: unhandled node type %T", n)
	}
}

func evalIdentifier(name string, ctx *Context) (Value, error) {
	switch name {
	case "time":
		return intValue(float64(ctx.Time)), nil
	case "stateno":
		return intValue(float64(ctx.StateNo)), nil
	case "anim":
		return intValue(float64(ctx.Anim)), nil
	case "animtime":
		return intValue(float64(animTime(ctx))), nil
	case "animelem":
		return Value{}, fmt.Errorf("evaluator: AnimElem must be compared to an element number, e.g. AnimElem = 3")
	case "ctrl":
		return boolValue(ctx.Ctrl), nil
	case "hitshakeover":
		return boolValue(ctx.GetHit.HitShakeTime <= 0), nil
	case "hitover":
		return boolValue(!ctx.GetHit.Active || ctx.GetHit.HitTime < 0), nil
	case "command":
		return Value{}, fmt.Errorf(`evaluator: Command must be compared to a quoted command name, e.g. Command = "holdback"`)
	case "statetype", "movetype", "p2statetype", "p2movetype":
		return Value{}, fmt.Errorf("evaluator: %s must be compared to a state letter, e.g. StateType = S", name)
	case "hitdefattr":
		return Value{}, fmt.Errorf("evaluator: HitDefAttr must be compared to an attribute, e.g. HitDefAttr = SCA, NA")
	case "pos", "vel", "p2dist", "p2bodydist":
		return Value{}, fmt.Errorf("evaluator: %s needs an axis, e.g. %s X", name, name)
	default:
		if v, ok, err := evalFighterTrigger(name, ctx); ok {
			return v, err
		}
		return Value{}, &UnknownTriggerError{Name: name}
	}
}

func evalCall(n callNode, ctx *Context) (Value, error) {
	switch n.name {
	case "var":
		return evalIndexedVar(n, ctx, "var", ctx.Vars[:])
	case "sysvar":
		return evalIndexedVar(n, ctx, "sysvar", ctx.SysVars[:])
	case "fvar":
		return evalIndexedFloatVar(n, ctx, "fvar", ctx.FVars[:])
	case "sysfvar":
		return evalIndexedFloatVar(n, ctx, "sysfvar", ctx.SysFVars[:])
	case "const":
		return evalConst(n, ctx)
	case "gethitvar":
		return evalGetHitVar(n, ctx)
	case "ifelse":
		if len(n.args) != 3 {
			return Value{}, fmt.Errorf("evaluator: IfElse() expects 3 arguments, got %d", len(n.args))
		}
		// All three arguments are evaluated, in order, before one is chosen,
		// as Ikemen GO compiles IfElse (compiler.go:1852-1886): a Random in
		// the branch not taken still draws.
		cond, err := evalNode(n.args[0], ctx)
		if err != nil {
			return Value{}, err
		}
		then, err := evalNode(n.args[1], ctx)
		if err != nil {
			return Value{}, err
		}
		otherwise, err := evalNode(n.args[2], ctx)
		if err != nil {
			return Value{}, err
		}
		if cond.Bool() {
			return then, nil
		}
		return otherwise, nil
	default:
		if v, ok, err := evalFighterCall(n, ctx); ok {
			return v, err
		}
		return Value{}, &UnknownTriggerError{Name: n.name, Call: true}
	}
}

// evalGetHitVar reads GetHitVar(name). Like Command, its argument is a name
// read as written, never evaluated; an unknown name is an error rather than
// a silent 0.
// unknownGetHitVarError is evalGetHitVar's rejection of a variable it does
// not handle; IsKnownTrigger recognizes it by type.
type unknownGetHitVarError struct{ name string }

func (e *unknownGetHitVarError) Error() string {
	return fmt.Sprintf("evaluator: GetHitVar: unknown variable %q", e.name)
}

func evalGetHitVar(n callNode, ctx *Context) (Value, error) {
	if len(n.args) != 1 {
		return Value{}, fmt.Errorf("evaluator: GetHitVar() expects 1 argument, got %d", len(n.args))
	}
	id, ok := n.args[0].(identifierNode)
	if !ok {
		return Value{}, fmt.Errorf("evaluator: GetHitVar() expects a variable name")
	}
	g := ctx.GetHit
	switch id.name {
	case "hittime":
		return intValue(float64(g.HitTime)), nil
	case "slidetime":
		return intValue(float64(g.SlideTime)), nil
	case "hitshaketime":
		return intValue(float64(g.HitShakeTime)), nil
	case "animtype":
		return intValue(float64(g.AnimType)), nil
	case "groundtype":
		return intValue(float64(g.GroundType)), nil
	case "airtype":
		return intValue(float64(g.AirType)), nil
	case "ctrltime":
		return intValue(float64(g.CtrlTime)), nil
	case "yvel":
		return floatValue(g.Yvel), nil
	case "yaccel":
		return floatValue(g.Yaccel), nil
	case "fall":
		return boolValue(g.Fall), nil
	case "fall.yvel":
		return floatValue(g.FallYvel), nil
	case "fall.recover":
		return boolValue(g.FallRecover), nil
	case "fall.recovertime":
		return intValue(float64(g.FallRecoverTime)), nil
	case "isbound":
		// bytecode.go:3511. shortcut: nothing binds a fighter yet (BindToTarget,
		// backlog item 029 area), so it is never bound.
		return boolValue(false), nil
	default:
		return Value{}, &unknownGetHitVarError{name: id.name}
	}
}

// evalVarIndex evaluates the single index argument of var(N), fvar(N) and
// their system forms and checks it against the slot count.
func evalVarIndex(n callNode, ctx *Context, fnName string, size int) (int, error) {
	if len(n.args) != 1 {
		return 0, fmt.Errorf("evaluator: %s() expects 1 argument, got %d", fnName, len(n.args))
	}
	idxVal, err := evalNode(n.args[0], ctx)
	if err != nil {
		return 0, err
	}
	idx := idxVal.Int()
	return idx, checkSlot(fnName, idx, size)
}

// checkSlot reports an index outside the size slots of variable function name.
func checkSlot(name string, idx, size int) error {
	if idx < 0 || idx >= size {
		return fmt.Errorf("evaluator: %s index %d out of range 0-%d", name, idx, size-1)
	}
	return nil
}

func evalIndexedVar(n callNode, ctx *Context, fnName string, slot []int) (Value, error) {
	idx, err := evalVarIndex(n, ctx, fnName, len(slot))
	if err != nil {
		return Value{}, err
	}
	return intValue(float64(slot[idx])), nil
}

func evalIndexedFloatVar(n callNode, ctx *Context, fnName string, slot []float32) (Value, error) {
	idx, err := evalVarIndex(n, ctx, fnName, len(slot))
	if err != nil {
		return Value{}, err
	}
	return floatValue(float64(slot[idx])), nil
}

var errAssignNeedsMut = errors.New("evaluator: an expression with := needs EvalMut/EvaluateMut: Eval works on a copy and would drop the write")

// AssignVar stores v in var(idx), sysvar(idx), fvar(idx) or sysfvar(idx) (target
// names the function, lower-case) and returns the stored value, as Ikemen
// GO's varSet family does (char.go:8325-8404): an int variable keeps v
// truncated, a float one keeps it as float32. An index outside the slots is
// an error.
func (c *Context) AssignVar(target string, idx int, v Value) (Value, error) {
	switch target {
	case "var":
		if err := checkSlot(target, idx, len(c.Vars)); err != nil {
			return Value{}, err
		}
		c.Vars[idx] = v.Int()
		return intValue(float64(c.Vars[idx])), nil
	case "sysvar":
		if err := checkSlot(target, idx, len(c.SysVars)); err != nil {
			return Value{}, err
		}
		c.SysVars[idx] = v.Int()
		return intValue(float64(c.SysVars[idx])), nil
	case "fvar":
		if err := checkSlot(target, idx, len(c.FVars)); err != nil {
			return Value{}, err
		}
		c.FVars[idx] = float32(v.Number)
		return floatValue(float64(c.FVars[idx])), nil
	case "sysfvar":
		if err := checkSlot(target, idx, len(c.SysFVars)); err != nil {
			return Value{}, err
		}
		c.SysFVars[idx] = float32(v.Number)
		return floatValue(float64(c.SysFVars[idx])), nil
	}
	return Value{}, fmt.Errorf("evaluator: %q is not a variable", target)
}

// evalAssign runs "var(i) := v" and friends, in Ikemen GO's order (index
// pushed first, then the value; compiler.go:1331-1345) and returns the
// stored value as char.go:8333 varSet does.
func evalAssign(n assignNode, ctx *Context) (Value, error) {
	idxVal, err := evalNode(n.index, ctx)
	if err != nil {
		return Value{}, err
	}
	value, err := evalNode(n.value, ctx)
	if err != nil {
		return Value{}, err
	}
	return ctx.AssignVar(n.target, idxVal.Int(), value)
}

// evalConst reads Const(name). Like GetHitVar, its argument is a name read as
// written (a dotted identifier), never evaluated. An unknown name reads int 0
// rather than failing, as Ikemen GO's does (compiler.go:2518-2521).
func evalConst(n callNode, ctx *Context) (Value, error) {
	if len(n.args) != 1 {
		return Value{}, fmt.Errorf("evaluator: Const() expects 1 argument, got %d", len(n.args))
	}
	id, ok := n.args[0].(identifierNode)
	if !ok {
		return Value{}, fmt.Errorf("evaluator: Const() expects a constant name, e.g. Const(movement.yaccel)")
	}
	return ctx.Consts.Lookup(id.name), nil
}

func evalLocal(name string, ctx *Context) (Value, error) {
	for i, n := range ctx.Locals.Names {
		if n == name && i < len(ctx.Locals.Values) {
			return ctx.Locals.Values[i], nil
		}
	}
	return Value{}, fmt.Errorf("evaluator: unknown local variable %q", "$"+name)
}

func evalUnary(n unaryNode, ctx *Context) (Value, error) {
	operand, err := evalNode(n.operand, ctx)
	if err != nil {
		return Value{}, err
	}
	switch n.op {
	case "!":
		return boolValue(!operand.Bool()), nil
	case "-":
		if operand.Kind == KindFloat {
			return floatValue(-operand.Number), nil
		}
		return intValue(-operand.Number), nil
	default:
		return Value{}, fmt.Errorf("evaluator: internal error: unhandled unary operator %q", n.op)
	}
}

func evalBinary(n binaryNode, ctx *Context) (Value, error) {
	if n.op == "=" || n.op == "!=" {
		if v, ok, err := evalTypeComparison(n, ctx); ok {
			return v, err
		}
		if cmdName, ok := commandComparisonTarget(n); ok {
			active := ctx.ActiveCommands[cmdName]
			if n.op == "!=" {
				active = !active
			}
			return boolValue(active), nil
		}
	}

	left, err := evalNode(n.left, ctx)
	if err != nil {
		return Value{}, err
	}
	// A CNS trigger evaluates both operands of && and || (Ikemen GO
	// compiles them as OC_bland/OC_blor inside a state controller,
	// compiler.go:6042-6045, 6086-6090), so a Random on the right draws
	// even when the left decides; a .zss expression stops early.
	if n.shortCircuit && n.op == "&&" && !left.Bool() {
		return boolValue(false), nil
	}
	if n.shortCircuit && n.op == "||" && left.Bool() {
		return boolValue(true), nil
	}
	right, err := evalNode(n.right, ctx)
	if err != nil {
		return Value{}, err
	}

	if v, ok, err := evalComparisonOrLogical(n.op, left, right); ok || err != nil {
		return v, err
	}
	return evalArithmetic(n.op, left, right)
}

// evalComparisonOrLogical handles op's comparison (=, !=, <, >, <=, >=) and
// logical (&&, ||) forms. ok is false, with a zero Value and nil error,
// when op is not one of those -- the caller falls through to
// evalArithmetic instead.
func evalComparisonOrLogical(op string, left, right Value) (v Value, ok bool, err error) {
	switch op {
	case "=":
		return boolValue(left.Float() == right.Float()), true, nil
	case "!=":
		return boolValue(left.Float() != right.Float()), true, nil
	case "<":
		return boolValue(left.Float() < right.Float()), true, nil
	case ">":
		return boolValue(left.Float() > right.Float()), true, nil
	case "<=":
		return boolValue(left.Float() <= right.Float()), true, nil
	case ">=":
		return boolValue(left.Float() >= right.Float()), true, nil
	case "&&":
		return boolValue(left.Bool() && right.Bool()), true, nil
	case "||":
		return boolValue(left.Bool() || right.Bool()), true, nil
	default:
		return Value{}, false, nil
	}
}

// evalArithmetic handles op's arithmetic form (+, -, *, /, %) -- every
// binary operator evalComparisonOrLogical does not recognize.
func evalArithmetic(op string, left, right Value) (Value, error) {
	switch op {
	case "+":
		return arithResult(left, right, left.Float()+right.Float()), nil
	case "-":
		return arithResult(left, right, left.Float()-right.Float()), nil
	case "*":
		return arithResult(left, right, left.Float()*right.Float()), nil
	case "/":
		if right.Float() == 0 {
			return Value{}, fmt.Errorf("evaluator: division by zero")
		}
		return floatValue(left.Float() / right.Float()), nil
	case "%":
		if right.Float() == 0 {
			return Value{}, fmt.Errorf("evaluator: modulo by zero")
		}
		return arithResult(left, right, math.Mod(left.Float(), right.Float())), nil
	default:
		return Value{}, fmt.Errorf("evaluator: internal error: unhandled binary operator %q", op)
	}
}

// arithResult applies MUGEN's type-propagation rule for arithmetic: the
// result is float-kinded if either operand is, otherwise int-kinded.
func arithResult(left, right Value, result float64) Value {
	if left.Kind == KindFloat || right.Kind == KindFloat {
		return floatValue(result)
	}
	return intValue(result)
}

// commandComparisonTarget recognizes MUGEN's special-form Command trigger:
// "Command = \"name\"" (or "!="), where Command is not a generically
// evaluable identifier but a redirect checked against the currently
// recognized input commands (see Context.ActiveCommands). It returns the
// compared-to command name and true only when the binary node is exactly
// that form, in either operand order.
func commandComparisonTarget(n binaryNode) (string, bool) {
	if id, ok := n.left.(identifierNode); ok && strings.EqualFold(id.name, "command") {
		if s, ok := n.right.(stringNode); ok {
			return s.value, true
		}
	}
	if id, ok := n.right.(identifierNode); ok && strings.EqualFold(id.name, "command") {
		if s, ok := n.left.(stringNode); ok {
			return s.value, true
		}
	}
	return "", false
}
