package evaluator

// node is one element of a parsed expression's abstract syntax tree. It
// carries no behavior of its own — eval (in eval.go) walks it to produce a
// Value, matching the rest of this package's separation between parsing
// (lexer.go/parser.go) and evaluation (eval.go).
type node interface {
	isNode()
}

// numberNode is a numeric literal, e.g. "10" or "3.5". isFloat records
// whether the source text contained a decimal point, so evaluation can
// report the MUGEN-typed result (int vs float) the literal actually is.
type numberNode struct {
	value   float64
	isFloat bool
}

// stringNode is a double-quoted string literal, e.g. "holdback". Only
// meaningful as an operand of the Command trigger (see eval.go); evaluated
// in any other position it is a descriptive error, not a panic.
type stringNode struct {
	value string
}

// identifierNode is a bare trigger name, e.g. Time, Ctrl, StateNo,
// Command, or an unrecognized name — unknown names are resolved to a
// descriptive error at evaluation time, not at parse time. name is
// lower-cased by the parser.
type identifierNode struct {
	name string
}

// callNode is a function-style trigger, e.g. var(0), sysvar(1),
// IfElse(cond, a, b), or an unrecognized function name (name is
// lower-cased by the parser).
type callNode struct {
	name string
	args []node
}

// unaryNode is a prefix operator applied to a single operand: "!" (logical
// not), "-" (numeric negation) or "~" (bitwise not).
type unaryNode struct {
	op      string
	operand node
}

// binaryNode is an infix operator applied to two operands: comparison
// (=, !=, <, >, <=, >=), logical (&&, ||, ^^), bitwise (&, |, ^), or
// arithmetic (+, -, *, /, %, **).
type binaryNode struct {
	op          string
	left, right node
	// shortCircuit, set only on && and || parsed by ParseShortCircuit,
	// skips the right operand when the left decides the result.
	shortCircuit bool
}

// componentNode is an axis-qualified trigger such as "Pos Y" or "Vel X":
// the trigger name (lower-cased) followed by a bare axis letter. The parser
// builds it only for the names in axisTriggers.
type componentNode struct {
	name string
	axis byte // 'x', 'y' or 'z'
}

// localNode is a local variable read, "$name" (name lower-cased).
type localNode struct {
	name string
}

// assignNode is "var(i) := v", "sysvar(i) := v", "fvar(i) := v" or
// "sysfvar(i) := v": target is the lower-cased variable function name. Its
// value is the assigned value.
type assignNode struct {
	target string
	index  node
	value  node
}

// animElemNode is "AnimElem = n" or "AnimElem = n, op t": the AnimElemTime(n)
// comparison Ikemen GO compiles it to. op is "=" when the source has no
// comparison after the comma, and value is t, 0 when there is no comma.
//
// With interval set it is "AnimElem = n, = [lo, hi]" (or "!=", not set):
// AnimElemTime(n) checked against integer bounds, op and value unused.
type animElemNode struct {
	elem  int
	op    string
	value node

	interval       bool
	not            bool
	lo, hi         int
	loOpen, hiOpen bool
}

// intervalNode is "value = [lo, hi]" or "value != [lo, hi]", each bound
// included with "[" "]" and excluded with "(" ")"; not is set for !=.
type intervalNode struct {
	value, lo, hi  node
	loOpen, hiOpen bool
	not            bool
}

func (animElemNode) isNode()   {}
func (intervalNode) isNode()   {}
func (localNode) isNode()      {}
func (assignNode) isNode()     {}
func (componentNode) isNode()  {}
func (numberNode) isNode()     {}
func (stringNode) isNode()     {}
func (identifierNode) isNode() {}
func (callNode) isNode()       {}
func (unaryNode) isNode()      {}
func (binaryNode) isNode()     {}
