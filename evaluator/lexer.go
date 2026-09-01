package evaluator

import (
	"fmt"
	"strings"
)

// tokenKind classifies a single lexical token produced by lex.
type tokenKind int

const (
	tokEOF tokenKind = iota
	tokNumber
	tokString
	tokIdent
	tokLParen
	tokRParen
	tokComma
	tokOp
)

// token is one lexical unit of a trigger/expression string. text holds the
// literal source text for tokNumber/tokIdent/tokOp, or the unquoted
// contents for tokString; it is empty for tokEOF/tokLParen/tokRParen/
// tokComma, whose kind alone is meaningful.
type token struct {
	kind tokenKind
	text string
}

// multiCharOps must be tried before single-character operators so that,
// e.g., "!=" lexes as one token rather than "!" followed by "=".
var multiCharOps = []string{"!=", "<=", ">=", "&&", "||"}

const singleCharOps = "=<>!+-*/%"

// lex turns a raw MUGEN CNS expression string into a flat token stream,
// terminated by a single tokEOF, or returns a descriptive error for an
// unterminated string literal or an unrecognized character — it never
// panics, including on empty or whitespace-only input (which lexes to just
// tokEOF).
func lex(input string) ([]token, error) {
	var toks []token
	i := 0
	n := len(input)

	for i < n {
		c := input[i]

		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++

		case c == '(':
			toks = append(toks, token{kind: tokLParen, text: "("})
			i++

		case c == ')':
			toks = append(toks, token{kind: tokRParen, text: ")"})
			i++

		case c == ',':
			toks = append(toks, token{kind: tokComma, text: ","})
			i++

		case c == '"':
			tok, next, err := scanString(input, i)
			if err != nil {
				return nil, err
			}
			toks = append(toks, tok)
			i = next

		case isDigit(c) || (c == '.' && i+1 < n && isDigit(input[i+1])):
			tok, next := scanNumber(input, i)
			toks = append(toks, tok)
			i = next

		case isIdentStart(c):
			tok, next := scanIdent(input, i)
			toks = append(toks, tok)
			i = next

		default:
			if op, ok := matchMultiCharOp(input[i:]); ok {
				toks = append(toks, token{kind: tokOp, text: op})
				i += len(op)
				continue
			}
			if strings.IndexByte(singleCharOps, c) >= 0 {
				toks = append(toks, token{kind: tokOp, text: string(c)})
				i++
				continue
			}
			return nil, fmt.Errorf("evaluator: unexpected character %q at position %d in %q", c, i, input)
		}
	}

	toks = append(toks, token{kind: tokEOF})
	return toks, nil
}

// scanString scans a double-quoted string literal starting at i (which
// must point at the opening '"'), returning the produced tokString token
// and the index just past the closing '"'. Returns a descriptive error,
// never a panic, for an unterminated literal.
func scanString(input string, i int) (token, int, error) {
	n := len(input)
	j := i + 1
	for j < n && input[j] != '"' {
		j++
	}
	if j >= n {
		return token{}, 0, fmt.Errorf("evaluator: unterminated string literal starting at position %d in %q", i, input)
	}
	return token{kind: tokString, text: input[i+1 : j]}, j + 1, nil
}

// scanNumber scans a numeric literal (integer or decimal) starting at i,
// returning the produced tokNumber token and the index just past it.
func scanNumber(input string, i int) (token, int) {
	n := len(input)
	j := i
	for j < n && isDigit(input[j]) {
		j++
	}
	if j < n && input[j] == '.' {
		j++
		for j < n && isDigit(input[j]) {
			j++
		}
	}
	return token{kind: tokNumber, text: input[i:j]}, j
}

// scanIdent scans an identifier starting at i, returning the produced
// tokIdent token and the index just past it.
func scanIdent(input string, i int) (token, int) {
	n := len(input)
	j := i
	for j < n && isIdentPart(input[j]) {
		j++
	}
	return token{kind: tokIdent, text: input[i:j]}, j
}

func matchMultiCharOp(rest string) (string, bool) {
	for _, op := range multiCharOps {
		if strings.HasPrefix(rest, op) {
			return op, true
		}
	}
	return "", false
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || isDigit(c) || c == '.'
}
