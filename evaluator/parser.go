package evaluator

import (
	"fmt"
	"strconv"
	"strings"
)

// Expression is a parsed, evaluable MUGEN CNS trigger/parameter
// expression, as found verbatim in a character.cns Controller's Triggers
// or Parameters (see character/cns's Controller.Triggers/Parameters).
// Parse it once and call Eval as many times as needed — Expression itself
// holds no per-fighter state.
type Expression struct {
	root node
	// hasAssign reports a := inside: Eval, which works on a copy of its
	// Context, refuses such an expression instead of dropping the write.
	hasAssign bool
}

// Parse parses a raw MUGEN CNS expression string into an Expression,
// honoring MUGEN's operator precedence (lowest to highest: ||, &&,
// comparisons, +/-, */÷/%, unary !/-). It returns a descriptive error —
// never a panic — for an empty, unbalanced, or otherwise malformed
// expression.
func Parse(expr string) (*Expression, error) {
	return parse(expr, false)
}

// ParseShortCircuit is Parse for a .zss expression: its && and || stop at
// the left operand when it decides the result, as Ikemen GO compiles them
// outside a CNS controller (compiler.go expBoolAnd/expBoolOr with no state
// block: OC_jz/OC_jnz jumps). A CNS trigger evaluates both operands.
func ParseShortCircuit(expr string) (*Expression, error) {
	return parse(expr, true)
}

func parse(expr string, shortCircuit bool) (*Expression, error) {
	toks, err := lex(expr)
	if err != nil {
		return nil, err
	}

	p := &parser{toks: toks, shortCircuit: shortCircuit}
	if p.peek().kind == tokEOF {
		return nil, fmt.Errorf("evaluator: empty expression")
	}

	root, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tokEOF {
		return nil, fmt.Errorf("evaluator: unexpected token %q after expression %q", p.peek().text, expr)
	}

	return &Expression{root: root, hasAssign: p.hasAssign}, nil
}

type parser struct {
	toks []token
	pos  int
	// shortCircuit marks the && and || nodes it builds as short-circuiting
	// (see binaryNode.shortCircuit).
	shortCircuit bool
	hasAssign    bool
}

func (p *parser) peek() token {
	return p.toks[p.pos]
}

func (p *parser) next() token {
	t := p.toks[p.pos]
	if p.pos < len(p.toks)-1 {
		p.pos++
	}
	return t
}

func (p *parser) peekIsOp(ops ...string) bool {
	t := p.peek()
	if t.kind != tokOp {
		return false
	}
	for _, op := range ops {
		if t.text == op {
			return true
		}
	}
	return false
}

func (p *parser) parseOr() (node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peekIsOp("||") {
		op := p.next().text
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op, left: left, right: right, shortCircuit: p.shortCircuit}
	}
	return left, nil
}

func (p *parser) parseAnd() (node, error) {
	left, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	for p.peekIsOp("&&") {
		op := p.next().text
		right, err := p.parseComparison()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op, left: left, right: right, shortCircuit: p.shortCircuit}
	}
	return left, nil
}

func (p *parser) parseComparison() (node, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	for p.peekIsOp("=", "!=", "<", ">", "<=", ">=") {
		op := p.next().text
		right, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseAdditive() (node, error) {
	left, err := p.parseMultiplicative()
	if err != nil {
		return nil, err
	}
	for p.peekIsOp("+", "-") {
		op := p.next().text
		right, err := p.parseMultiplicative()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseMultiplicative() (node, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peekIsOp("*", "/", "%") {
		op := p.next().text
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = binaryNode{op: op, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (node, error) {
	if p.peekIsOp("!", "-") {
		op := p.next().text
		operand, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return unaryNode{op: op, operand: operand}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (node, error) {
	t := p.peek()
	switch t.kind {
	case tokNumber:
		p.next()
		v, err := strconv.ParseFloat(t.text, 64)
		if err != nil {
			return nil, fmt.Errorf("evaluator: invalid numeric literal %q: %w", t.text, err)
		}
		return numberNode{value: v, isFloat: strings.Contains(t.text, ".")}, nil

	case tokString:
		p.next()
		return stringNode{value: t.text}, nil

	case tokIdent:
		p.next()
		return p.parseIdentifierOrCall(t.text)

	case tokLocal:
		p.next()
		name := strings.ToLower(t.text)
		if !validLocalName(name) || name == "_" {
			return nil, fmt.Errorf("evaluator: invalid local variable name %q", "$"+t.text)
		}
		if p.peekIsOp(":=") {
			return nil, fmt.Errorf("evaluator: local variable assignment with := is not possible in an expression (%q)", "$"+t.text)
		}
		return localNode{name: name}, nil

	case tokLParen:
		p.next()
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tokRParen {
			return nil, fmt.Errorf("evaluator: expected ')' but found %q", p.peek().text)
		}
		p.next()
		return inner, nil

	case tokEOF:
		return nil, fmt.Errorf("evaluator: unexpected end of expression")

	default:
		return nil, fmt.Errorf("evaluator: unexpected token %q", t.text)
	}
}

// parseIdentifierOrCall resolves an already-consumed identifier token
// (name) into either a plain identifierNode, or -- if immediately followed
// by '(' -- a callNode wrapping its parsed argument list. The node's name is
// lower-cased here, once, so evaluation never has to (MUGEN trigger names
// are case-insensitive).
func (p *parser) parseIdentifierOrCall(name string) (node, error) {
	if axisTriggers[strings.ToLower(name)] && p.peek().kind == tokIdent {
		axis := strings.ToLower(p.peek().text)
		if axis == "x" || axis == "y" || axis == "z" {
			p.next()
			return componentNode{name: strings.ToLower(name), axis: axis[0]}, nil
		}
	}
	if strings.EqualFold(name, "animelem") && p.peekIsOp("=", "!=", "<", ">", "<=", ">=") {
		return p.parseAnimElem()
	}
	if strings.EqualFold(name, "hitdefattr") && p.peekIsOp("=", "!=") {
		return p.parseHitDefAttr()
	}
	if _, isRedirect := redirectKinds[strings.ToLower(name)]; isRedirect {
		return p.parseRedirect(strings.ToLower(name))
	}
	if p.peek().kind != tokLParen {
		return identifierNode{name: strings.ToLower(name)}, nil
	}

	args, err := p.parseCallArgs(name)
	if err != nil {
		return nil, err
	}
	call := callNode{name: strings.ToLower(name), args: args}
	if !p.peekIsOp(":=") {
		return call, nil
	}
	switch call.name {
	case "var", "sysvar", "fvar", "sysfvar":
	default:
		return nil, fmt.Errorf("evaluator: %q cannot be assigned with :=; only var, sysvar, fvar and sysfvar can", name)
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("evaluator: %s() expects 1 argument, got %d", call.name, len(args))
	}
	p.next() // consume ':='
	// The right side is Ikemen GO's expEqne (compiler.go:1331-1340): a
	// comparison or tighter, not && or ||.
	value, err := p.parseComparison()
	if err != nil {
		return nil, err
	}
	p.hasAssign = true
	return assignNode{target: call.name, index: args[0], value: value}, nil
}

// validLocalName reports whether name matches [a-z_][0-9a-z_]*, Ikemen GO's
// varNameCheck (compiler.go:7695).
func validLocalName(name string) bool {
	if name == "" || (name[0] != '_' && (name[0] < 'a' || name[0] > 'z')) {
		return false
	}
	for i := 1; i < len(name); i++ {
		c := name[i]
		if c != '_' && (c < 'a' || c > 'z') && !isDigit(c) {
			return false
		}
	}
	return true
}

// parseCallArgs parses a parenthesized, comma-separated argument list for a
// call to name, starting at the '(' peek() must currently be positioned on
// and consuming through the matching ')'.
func (p *parser) parseCallArgs(name string) ([]node, error) {
	p.next() // consume '('
	var args []node
	if p.peek().kind != tokRParen {
		for {
			arg, err := p.parseOr()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if p.peek().kind == tokComma {
				p.next()
				continue
			}
			break
		}
	}
	if p.peek().kind != tokRParen {
		return nil, fmt.Errorf("evaluator: expected ')' to close call to %q", name)
	}
	p.next() // consume ')'
	return args, nil
}

// parseAnimElem parses what follows the AnimElem trigger: "= n" and an
// optional ", op t" (Ikemen GO's old-style trigger, compiler.go:1296-1311 and
// 4297-4318). The first comparison is "=" with a positive integer; after the
// comma the operator may be omitted, meaning "=".
func (p *parser) parseAnimElem() (node, error) {
	if op := p.next().text; op != "=" {
		return nil, fmt.Errorf("evaluator: AnimElem only supports '=' before the element number, not %q", op)
	}
	tok := p.peek()
	elem, err := strconv.Atoi(tok.text)
	if tok.kind != tokNumber || err != nil {
		return nil, fmt.Errorf("evaluator: AnimElem expects an integer element number, got %q", tok.text)
	}
	if elem <= 0 {
		return nil, fmt.Errorf("evaluator: AnimElem must be greater than 0")
	}
	p.next()
	if p.peek().kind != tokComma {
		return animElemNode{elem: elem, op: "=", value: numberNode{}}, nil
	}
	p.next()
	op := "="
	if p.peekIsOp("=", "!=", "<", ">", "<=", ">=") {
		op = p.next().text
	}
	value, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}
	return animElemNode{elem: elem, op: op, value: value}, nil
}

// parseHitDefAttr parses what follows the HitDefAttr trigger: "= attr" or
// "!= attr", the attr being a word of state types and any number of attack
// classes after commas, bare or in quotes (Ikemen GO compiler.go:1510-1535,
// 830-897). A comma followed by a word that is no attack class ends the
// attribute, as trgAttr stops there.
func (p *parser) parseHitDefAttr() (node, error) {
	not := p.next().text == "!="
	tok := p.next()
	var text string
	switch tok.kind {
	case tokString:
		text = tok.text
	case tokIdent:
		text = tok.text
		for p.peek().kind == tokComma && p.toks[p.pos+1].kind == tokIdent {
			if _, ok := attackClass(p.toks[p.pos+1].text); !ok {
				break
			}
			p.next()
			text += "," + p.next().text
		}
	default:
		return nil, fmt.Errorf("evaluator: HitDefAttr expects an attribute such as SCA, NA after the operator, got %q", tok.text)
	}
	attr, err := ParseAttr(text, false)
	if err != nil {
		return nil, err
	}
	return hitDefAttrNode{attr: attr, not: not}, nil
}

// parseRedirect parses a redirection keyword already consumed: an optional
// "(index)", a comma and the trigger it redirects (Ikemen GO compiler.go:
// 836-883). The redirection reaches only the next primary, so "p2,time > 1"
// is (p2,time) > 1, and a state or move type comparison belongs to it. A
// redirection to a helper or the like fails at parse time, naming itself.
func (p *parser) parseRedirect(kind string) (node, error) {
	var arg node
	if p.peek().kind == tokLParen {
		args, err := p.parseCallArgs(kind)
		if err != nil {
			return nil, err
		}
		if len(args) != 1 {
			return nil, fmt.Errorf("evaluator: %s() expects 1 argument, got %d", kind, len(args))
		}
		arg = args[0]
	}
	if p.peek().kind != tokComma {
		return nil, fmt.Errorf("evaluator: %s must be followed by ',' and a trigger, e.g. %s, Life", kind, kind)
	}
	p.next()
	if !redirectKinds[kind] {
		return nil, fmt.Errorf("evaluator: redirection to %s is not supported yet: the engine has no helpers", kind)
	}
	inner, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	if id, ok := inner.(identifierNode); ok && (id.name == "statetype" || id.name == "movetype") && p.peekIsOp("=", "!=") {
		op := p.next().text
		letter := p.next()
		if letter.kind != tokIdent {
			return nil, fmt.Errorf("evaluator: %s must be compared to a state letter, got %q", id.name, letter.text)
		}
		inner = binaryNode{op: op, left: id, right: identifierNode{name: strings.ToLower(letter.text)}}
	}
	return redirectNode{kind: kind, arg: arg, inner: inner}, nil
}
