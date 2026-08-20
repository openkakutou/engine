package zssexec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/openkakutou/character/cns"
)

// stmt is one parsed .zss body statement -- ifStmt, callStmt, or ctrlStmt.
type stmt interface {
	isStmt()
}

// ifStmt is an "if COND { ... } [else { ... }]" statement. els is nil when
// no else branch is present.
type ifStmt struct {
	cond string
	then []stmt
	els  []stmt
}

func (ifStmt) isStmt() {}

// callStmt is a "call FunctionName();" statement.
type callStmt struct {
	name string
}

func (callStmt) isStmt() {}

// ctrlStmt is a single-line "Name{key: value; ...}" controller-call
// statement, holding the same cns.Controller shape statemachine already
// knows how to apply.
type ctrlStmt struct {
	ctrl cns.Controller
}

func (ctrlStmt) isStmt() {}

var (
	// ifHeaderPattern matches an "if COND {" line, capturing COND.
	ifHeaderPattern = regexp.MustCompile(`^if\s+(.+?)\s*\{$`)
	// callPattern matches a "call Name();" line (only an empty parameter
	// list is supported -- see the package doc comment).
	callPattern = regexp.MustCompile(`^call\s+([A-Za-z_]\w*)\s*\(\s*\)\s*;?$`)
	// callWithArgsPattern recognizes a "call Name(...)" line with a
	// non-empty argument list, so it can be reported as an explicit
	// unsupported construct rather than falling through to the generic
	// "unrecognized line" error.
	callWithArgsPattern = regexp.MustCompile(`^call\s+([A-Za-z_]\w*)\s*\(.+\)\s*;?$`)
	// ctrlStmtPattern matches a "Name{params};" controller-call statement.
	ctrlStmtPattern = regexp.MustCompile(`^([A-Za-z_]\w*)\s*\{(.*)\}\s*;?$`)
)

// parseBody parses a .zss block's raw Body text (character/zss.Block.Body)
// into a sequence of statements, in source order.
func parseBody(body string) ([]stmt, error) {
	lines := bodyLines(body)
	stmts, rest, err := parseStmts(lines)
	if err != nil {
		return nil, err
	}
	if len(rest) > 0 {
		return nil, fmt.Errorf("unexpected %q with no matching \"if\"/\"else\" block open", rest[0])
	}
	return stmts, nil
}

// bodyLines splits body into trimmed, non-blank lines, one statement/brace
// token per line. A "} else {" line (a closing then-brace and an opening
// else-brace written on the same source line, as real .zss/C-style
// formatting commonly does) is split into its own "}" and "else {" lines
// so the rest of the parser only ever has to recognize one brace token per
// line.
func bodyLines(body string) []string {
	var lines []string
	for _, l := range strings.Split(body, "\n") {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if l == "} else {" {
			lines = append(lines, "}", "else {")
			continue
		}
		lines = append(lines, l)
	}
	return lines
}

// parseStmts consumes statements from the front of lines until it either
// runs out of input or reaches a line that closes the enclosing block
// ("}") or starts an else branch ("else ..."), returning the parsed
// statements plus whatever lines were not consumed.
func parseStmts(lines []string) ([]stmt, []string, error) {
	var stmts []stmt
	for len(lines) > 0 {
		line := lines[0]
		if line == "}" || strings.HasPrefix(line, "else") {
			return stmts, lines, nil
		}

		var (
			s   stmt
			err error
		)
		switch {
		case strings.HasPrefix(line, "if "):
			s, lines, err = parseIf(lines)
		case strings.HasPrefix(line, "call "):
			s, err = parseCall(line)
			lines = lines[1:]
		default:
			s, err = parseCtrlStmt(line)
			lines = lines[1:]
		}
		if err != nil {
			return nil, nil, err
		}
		stmts = append(stmts, s)
	}
	return stmts, lines, nil
}

// parseIf parses an "if COND { ... } [else { ... }]" statement starting at
// lines[0], returning the parsed statement and the lines remaining after
// its closing brace(s).
func parseIf(lines []string) (stmt, []string, error) {
	m := ifHeaderPattern.FindStringSubmatch(lines[0])
	if m == nil {
		return nil, nil, fmt.Errorf("malformed \"if\" statement: %q", lines[0])
	}
	cond := m[1]

	then, rest, err := parseStmts(lines[1:])
	if err != nil {
		return nil, nil, err
	}
	if len(rest) == 0 || rest[0] != "}" {
		return nil, nil, fmt.Errorf("\"if %s {\" is missing its closing \"}\"", cond)
	}
	rest = rest[1:]

	if len(rest) > 0 && strings.HasPrefix(rest[0], "else") {
		if rest[0] != "else {" {
			return nil, nil, fmt.Errorf("malformed \"else\" clause: %q", rest[0])
		}
		els, rest2, err := parseStmts(rest[1:])
		if err != nil {
			return nil, nil, err
		}
		if len(rest2) == 0 || rest2[0] != "}" {
			return nil, nil, fmt.Errorf("\"else {\" is missing its closing \"}\"")
		}
		return ifStmt{cond: cond, then: then, els: els}, rest2[1:], nil
	}

	return ifStmt{cond: cond, then: then}, rest, nil
}

func parseCall(line string) (stmt, error) {
	if m := callPattern.FindStringSubmatch(line); m != nil {
		return callStmt{name: m[1]}, nil
	}
	if m := callWithArgsPattern.FindStringSubmatch(line); m != nil {
		return nil, fmt.Errorf("call to %q with arguments is not supported yet: %q", m[1], line)
	}
	return nil, fmt.Errorf("unsupported .zss construct: %q", line)
}

func parseCtrlStmt(line string) (stmt, error) {
	m := ctrlStmtPattern.FindStringSubmatch(line)
	if m == nil {
		return nil, fmt.Errorf("unsupported .zss construct: %q", line)
	}
	return ctrlStmt{ctrl: cns.Controller{
		Type:       m[1],
		Parameters: parseCtrlParams(m[2]),
	}}, nil
}

// parseCtrlParams parses a controller statement's semicolon-separated
// "key: value" parameter list (e.g. `v: 3; value: 7`) into a map, keyed by
// parameter name, mirroring cns.Controller.Parameters's own shape. Each
// value is kept verbatim (including surrounding quotes, if any) since it
// is handed to evaluator.Evaluate unchanged, the same unevaluated-string
// convention cns.Controller.Parameters already follows.
func parseCtrlParams(raw string) map[string]string {
	params := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, ":")
		if !ok {
			continue
		}
		params[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return params
}
