// Package evaluator parses and evaluates MUGEN CNS trigger/parameter
// expressions — the strings character/cns's Controller deliberately leaves
// unevaluated (Controller.Triggers []string, Controller.Parameters
// map[string]string; see that package's decision 011). It supports
// comparisons (=, !=, <, >, <=, >=), boolean/arithmetic operators (&&, ||,
// +, -, *, /, %, unary !), and a growing set of built-in trigger names and
// functions: var(), sysvar(), IfElse(), Time, StateNo, Anim, AnimTime,
// Command, and Ctrl. Later engine items are expected to extend the
// built-in set incrementally as they need more of MUGEN's trigger
// vocabulary — this package deliberately does not attempt to implement it
// exhaustively up front.
package evaluator

import (
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
	e, ok := parseCache[expr]
	if !ok {
		var err error
		e, err = Parse(expr)
		if err != nil {
			return Value{}, err
		}
		parseCache[expr] = e
	}
	return e.Eval(ctx)
}

// Eval evaluates the parsed expression against ctx, returning MUGEN's
// loosely-typed result. It returns a descriptive error — never a panic —
// when the expression references an unsupported/unknown trigger name or
// function, uses a value in an unsupported way (e.g. a bare string
// literal), or hits a runtime error such as an out-of-range var() index.
func (e *Expression) Eval(ctx Context) (Value, error) {
	return evalNode(e.root, ctx)
}

func evalNode(n node, ctx Context) (Value, error) {
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

	case unaryNode:
		return evalUnary(tn, ctx)

	case binaryNode:
		return evalBinary(tn, ctx)

	default:
		return Value{}, fmt.Errorf("evaluator: internal error: unhandled node type %T", n)
	}
}

func evalIdentifier(name string, ctx Context) (Value, error) {
	switch strings.ToLower(name) {
	case "time":
		return intValue(float64(ctx.Time)), nil
	case "stateno":
		return intValue(float64(ctx.StateNo)), nil
	case "anim":
		return intValue(float64(ctx.Anim)), nil
	case "animtime":
		return intValue(float64(ctx.AnimTime)), nil
	case "ctrl":
		return boolValue(ctx.Ctrl), nil
	case "command":
		return Value{}, fmt.Errorf(`evaluator: Command must be compared to a quoted command name, e.g. Command = "holdback"`)
	default:
		return Value{}, fmt.Errorf("evaluator: unknown trigger %q", name)
	}
}

func evalCall(n callNode, ctx Context) (Value, error) {
	switch strings.ToLower(n.name) {
	case "var":
		return evalIndexedVar(n, ctx, "var", ctx.Vars[:])
	case "sysvar":
		return evalIndexedVar(n, ctx, "sysvar", ctx.SysVars[:])
	case "ifelse":
		if len(n.args) != 3 {
			return Value{}, fmt.Errorf("evaluator: IfElse() expects 3 arguments, got %d", len(n.args))
		}
		cond, err := evalNode(n.args[0], ctx)
		if err != nil {
			return Value{}, err
		}
		if cond.Bool() {
			return evalNode(n.args[1], ctx)
		}
		return evalNode(n.args[2], ctx)
	default:
		return Value{}, fmt.Errorf("evaluator: unknown trigger function %q", n.name)
	}
}

func evalIndexedVar(n callNode, ctx Context, fnName string, slot []int) (Value, error) {
	if len(n.args) != 1 {
		return Value{}, fmt.Errorf("evaluator: %s() expects 1 argument, got %d", fnName, len(n.args))
	}
	idxVal, err := evalNode(n.args[0], ctx)
	if err != nil {
		return Value{}, err
	}
	idx := idxVal.Int()
	if idx < 0 || idx >= len(slot) {
		return Value{}, fmt.Errorf("evaluator: %s index %d out of range 0-%d", fnName, idx, len(slot)-1)
	}
	return intValue(float64(slot[idx])), nil
}

func evalUnary(n unaryNode, ctx Context) (Value, error) {
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

func evalBinary(n binaryNode, ctx Context) (Value, error) {
	if n.op == "=" || n.op == "!=" {
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
