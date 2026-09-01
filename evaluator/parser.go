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
}

// Parse parses a raw MUGEN CNS expression string into an Expression,
// honoring MUGEN's operator precedence (lowest to highest: ||, &&,
// comparisons, +/-, */÷/%, unary !/-). It returns a descriptive error —
// never a panic — for an empty, unbalanced, or otherwise malformed
// expression.
func Parse(expr string) (*Expression, error) {
	toks, err := lex(expr)
	if err != nil {
		return nil, err
	}

	p := &parser{toks: toks}
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

	return &Expression{root: root}, nil
}

type parser struct {
	toks []token
	pos  int
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
		left = binaryNode{op: op, left: left, right: right}
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
		left = binaryNode{op: op, left: left, right: right}
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
// by '(' -- a callNode wrapping its parsed argument list.
func (p *parser) parseIdentifierOrCall(name string) (node, error) {
	if p.peek().kind != tokLParen {
		return identifierNode{name: name}, nil
	}

	args, err := p.parseCallArgs(name)
	if err != nil {
		return nil, err
	}
	return callNode{name: name, args: args}, nil
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
