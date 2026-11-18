package zssexec

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/evaluator"
)

// The parser mirrors Ikemen GO's .zss compiler (src/compiler.go, commit
// 54d4148a: stateBlock 8246, subBlock 7850, blockAttribSet 7795, callFunc
// 8105, letAssign 8179, stateCompileZSS 8395) over the whole block body as
// one continuous text, so statements may span lines. A statement ends at a
// ';' or '{' (or '}') outside strings and brackets, as in readSentence
// (7598-7660). Expression text stays a string for the evaluator (decision
// 008); see .vibe/decisions/022.

// maxLocals is Ikemen GO's limit of local variables per Statedef/Function
// (inclNumVars, compiler.go:7737).
const maxLocals = 256

// stmt is one parsed .zss body statement.
type stmt interface {
	isStmt()
}

// blockStmt is a block: `[attributes] if COND { ... } [else ...]` or a bare
// `[attributes] { ... }` (hasCond false). els is the else branch: another
// blockStmt, itself with a condition for an `else if`.
type blockStmt struct {
	hasCond bool
	cond    string
	// persistIdx is the index in Context.Persistent of the block's
	// persistent(N) counter, -1 when it has none; persistN is the value the
	// counter is set to after a run (math.MaxInt32 for N <= 0).
	persistIdx int
	persistN   int32
	// ignoreHitPause is Ikemen GO's StateBlock.ignorehitpause (the block
	// itself runs during a hit pause) and ctrlsIgnoreHitPause its
	// ctrlsIgnorehitpause (its inner statements do). Recorded only: running
	// them during a hit pause is the gap of decision 021.
	ignoreHitPause, ctrlsIgnoreHitPause bool
	body                                []stmt
	els                                 *blockStmt
}

func (*blockStmt) isStmt() {}

// letStmt is `let a, b = e1, e2;`. slots[i] is the local slot of target i,
// -1 for the discarding name `_`; exprs[i] is its expression.
type letStmt struct {
	slots []int
	exprs []string
}

func (letStmt) isStmt() {}

// callStmt is `call F(args);` or the call half of `let a, b = call F(args);`
// (rets are then the callers' slots, -1 for `_`).
type callStmt struct {
	name string
	args []string
	rets []int
}

func (callStmt) isStmt() {}

// exprStmt is an expression statement with an assignment, e.g.
// `sysVar(1) := 0;`.
type exprStmt struct {
	expr string
}

func (exprStmt) isStmt() {}

// ctrlStmt is a `Name{key: value; ...}` controller call, holding the same
// cns.Controller shape statemachine already knows how to apply.
type ctrlStmt struct {
	ctrl cns.Controller
}

func (ctrlStmt) isStmt() {}

// compiledBlock is a parsed Statedef or Function: err is set instead of the
// rest when its body does not parse.
type compiledBlock struct {
	desc  string // "Statedef 7" or "Function name", named in every error
	err   error
	stmts []stmt
	// names is the block's flat table of local slots: for a Function its
	// arguments, then its return values, then every let; for a Statedef the
	// lets. A call gets a fresh frame of len(names) values.
	names          []string
	nArgs, nRets   int
	numPersistents int
	// header is a Statedef's header converted to the cns.StateDef shape the
	// root package's state entry already takes; zero for a Function.
	header cns.StateDef
}

// compileBlock parses b's header and body. The returned block always has
// desc; err is set when parsing failed.
func compileBlock(b zss.Block) *compiledBlock {
	cb := &compiledBlock{}
	p := &parser{cb: cb, stateNo: b.Number}
	if b.Kind == zss.BlockKindFunction {
		cb.desc = "Function " + b.Name
		p.isFunc = true
	} else {
		raw := b.NumberRaw
		if raw == "" {
			raw = strconv.Itoa(b.Number)
		}
		cb.desc = "Statedef " + raw
		cb.header = statedefHeader(b)
	}
	if err := p.parseBlock(b); err != nil {
		cb.err = fmt.Errorf("%s: %w", cb.desc, err)
	}
	return cb
}

type parser struct {
	cb      *compiledBlock
	src     string
	pos     int
	isFunc  bool
	stateNo int
}

func (p *parser) parseBlock(b zss.Block) error {
	if p.isFunc {
		if err := p.declareHeader(b); err != nil {
			return err
		}
	}
	src, err := stripComments(b.Body)
	if err != nil {
		return err
	}
	p.src = src
	stmts, err := p.stmts(nil)
	if err != nil {
		return err
	}
	p.cb.stmts = stmts
	return nil
}

// declareHeader fills the slot table with a Function's arguments, then its
// return values (compiler.go:8509-8530).
func (p *parser) declareHeader(b zss.Block) error {
	for _, a := range b.Params {
		a = strings.ToLower(strings.TrimSpace(a))
		if err := checkVarName(a); err != nil {
			return err
		}
		if a != "_" && p.slot(a) >= 0 {
			return fmt.Errorf("duplicate name %q in the parameters", a)
		}
		if a == "_" {
			// `_` has a slot like any argument (compiler.go:8509-8519) but
			// no name to read.
			p.cb.names = append(p.cb.names, a)
			continue
		}
		if _, err := p.declare(a); err != nil {
			return err
		}
	}
	p.cb.nArgs = len(b.Params)
	for _, r := range b.Ret {
		r = strings.ToLower(strings.TrimSpace(r))
		if err := checkVarName(r); err != nil {
			return err
		}
		if r == "_" {
			return fmt.Errorf("the return value name is _")
		}
		if p.slot(r) >= 0 {
			return fmt.Errorf("duplicate name %q in the return values", r)
		}
		if _, err := p.declare(r); err != nil {
			return err
		}
	}
	p.cb.nRets = len(b.Ret)
	return nil
}

func checkVarName(n string) error {
	if n == "" {
		return fmt.Errorf("empty variable name")
	}
	for i, c := range n {
		letter := (c >= 'a' && c <= 'z') || c == '_'
		digit := c >= '0' && c <= '9'
		if !letter && !(digit && i > 0) {
			return fmt.Errorf("invalid variable name %q", n)
		}
	}
	return nil
}

// slot returns the slot of a declared local, or -1.
func (p *parser) slot(name string) int {
	for i, n := range p.cb.names {
		if n == name {
			return i
		}
	}
	return -1
}

// declare returns name's slot, allocating it on first use; "_" has none.
func (p *parser) declare(name string) (int, error) {
	if name == "_" {
		return -1, nil
	}
	if i := p.slot(name); i >= 0 {
		return i, nil
	}
	if len(p.cb.names) >= maxLocals {
		return 0, fmt.Errorf("exceeded %d local variable limit", maxLocals)
	}
	p.cb.names = append(p.cb.names, name)
	return len(p.cb.names) - 1, nil
}

// checkLocals requires every $name in expr to be declared so far. It scans
// the text for "$name" because the
// parser must accept expressions the evaluator cannot parse yet (ranges,
// "p2Dist x", ... -- backlog items 041-043); those fail only if evaluated.
func (p *parser) checkLocals(expr string) error {
	for i := 0; i < len(expr); i++ {
		switch expr[i] {
		case '"':
			i += strings.IndexByte(expr[i+1:], '"') + 1
		case '$':
			j := i + 1
			for j < len(expr) && isIdentByte(expr[j]) {
				j++
			}
			name := strings.ToLower(expr[i+1 : j])
			if name == "_" || p.slot(name) < 0 {
				return fmt.Errorf("unknown local variable $%s in %q", name, snip(expr))
			}
			i = j - 1
		}
	}
	return nil
}

// stripComments removes '#' comments (to the end of the line) outside
// strings, and rejects a string not closed on its line (readString,
// compiler.go:7589).
func stripComments(body string) (string, error) {
	var sb strings.Builder
	for _, line := range strings.Split(body, "\n") {
		for i := 0; i < len(line); i++ {
			switch line[i] {
			case '#':
				line = line[:i]
			case '"':
				j := strings.IndexByte(line[i+1:], '"')
				if j < 0 {
					return "", fmt.Errorf("string not closed in %q", snip(line))
				}
				i += j + 1
			}
		}
		sb.WriteString(line)
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}

// snip shortens text for an error message.
func snip(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 80 {
		return s[:80] + "..."
	}
	return s
}

func (p *parser) skipSpace() {
	for p.pos < len(p.src) && strings.IndexByte(" \t\r\n", p.src[p.pos]) >= 0 {
		p.pos++
	}
}

func isIdentByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// peekWord returns the lower-cased identifier at the cursor without
// consuming it ("" if the cursor is not on one).
func (p *parser) peekWord() string {
	p.skipSpace()
	end := p.pos
	for end < len(p.src) && isIdentByte(p.src[end]) {
		end++
	}
	return strings.ToLower(p.src[p.pos:end])
}

func (p *parser) word() string {
	w := p.peekWord()
	p.pos += len(w)
	return w
}

// sentence reads up to the next ';', '{' or '}' outside strings and
// brackets and consumes it, returning the trimmed text and the terminator
// (0 at the end of the body).
func (p *parser) sentence() (string, byte) {
	start, depth := p.pos, 0
	for ; p.pos < len(p.src); p.pos++ {
		c := p.src[p.pos]
		switch c {
		case '"':
			p.pos += strings.IndexByte(p.src[p.pos+1:], '"') + 1 // stripComments checked it closes
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ';', '{', '}':
			if depth == 0 {
				p.pos++
				return strings.TrimSpace(p.src[start : p.pos-1]), c
			}
		}
	}
	return strings.TrimSpace(p.src[start:]), 0
}

// splitTop splits text at commas outside strings and brackets.
func splitTop(text string) []string {
	var parts []string
	start, depth := 0, 0
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '"':
			i += strings.IndexByte(text[i+1:], '"') + 1
		case '(', '[':
			depth++
		case ')', ']':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(text[start:i]))
				start = i + 1
			}
		}
	}
	return append(parts, strings.TrimSpace(text[start:]))
}

// hasAssign reports whether text contains ":=" outside strings.
func hasAssign(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] == '"' {
			i += strings.IndexByte(text[i+1:], '"') + 1
		} else if text[i] == ':' && i+1 < len(text) && text[i+1] == '=' {
			return true
		}
	}
	return false
}

// stmts parses statements until the '}' closing parent (consumed), or until
// the end of the body when parent is nil.
func (p *parser) stmts(parent *blockStmt) ([]stmt, error) {
	var out []stmt
	for {
		p.skipSpace()
		if p.pos >= len(p.src) {
			if parent != nil {
				return nil, fmt.Errorf("missing \"}\" closing %s", describe(parent))
			}
			return out, nil
		}
		c := p.src[p.pos]
		if c == '}' {
			if parent == nil {
				return nil, fmt.Errorf("unexpected \"}\" with no open block")
			}
			p.pos++
			return out, nil
		}
		var (
			s   stmt
			err error
		)
		w := p.peekWord()
		switch {
		case c == '{' || w == "if" || w == "ignorehitpause" || w == "persistent":
			s, err = p.subBlock(parent)
		case w == "for" || w == "while" || w == "switch":
			err = fmt.Errorf("%q blocks are not supported yet", w)
		case w == "else":
			err = fmt.Errorf("\"else\" without a matching \"if\"")
		case w == "call":
			p.pos += len(w)
			s, err = p.call(nil)
		case w == "let":
			p.pos += len(w)
			s, err = p.let()
		default:
			s, err = p.simple()
		}
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
}

func describe(b *blockStmt) string {
	if b.hasCond {
		return fmt.Sprintf("\"if %s {\"", snip(b.cond))
	}
	return "a \"{\" block"
}

// subBlock parses a block statement (subBlock, compiler.go:7850), its
// ignorehitpause inherited from parent as Ikemen GO does, and propagates a
// block that runs during a hit pause up to parent.
func (p *parser) subBlock(parent *blockStmt) (*blockStmt, error) {
	inherit := parent != nil && parent.ctrlsIgnoreHitPause
	b, err := p.block(inherit)
	if err != nil {
		return nil, err
	}
	if parent != nil && b.ignoreHitPause {
		parent.ignoreHitPause = true
	}
	return b, nil
}

// block parses attributes, the optional `if COND`, the braces and any else
// chain.
func (p *parser) block(inherit bool) (*blockStmt, error) {
	b := &blockStmt{persistIdx: -1, persistN: 1}
	if p.isFunc {
		inherit = true // compiler.go:7852: functions inject ignorehitpause
	}
	if inherit {
		b.ignoreHitPause, b.ctrlsIgnoreHitPause = true, true
		if p.peekWord() == "ignorehitpause" {
			p.word()
		}
	}
	if err := p.attributes(b); err != nil {
		return nil, err
	}
	p.skipSpace()
	switch w := p.peekWord(); {
	case p.pos < len(p.src) && p.src[p.pos] == '{':
		p.pos++
	case w == "if":
		p.pos += len(w)
		cond, term := p.sentence()
		if term != '{' {
			return nil, fmt.Errorf("\"if %s\" is not followed by \"{\"", snip(cond))
		}
		if cond == "" {
			return nil, fmt.Errorf("\"if\" without a condition")
		}
		if err := p.checkLocals(cond); err != nil {
			return nil, err
		}
		b.hasCond, b.cond = true, cond
	case w == "for" || w == "while" || w == "switch":
		return nil, fmt.Errorf("%q blocks are not supported yet", w)
	default:
		return nil, fmt.Errorf("expected \"if\" or \"{\" after the block attributes, found %q", snip(p.rest()))
	}
	body, err := p.stmts(b)
	if err != nil {
		return nil, err
	}
	b.body = body
	if b.hasCond && p.peekWord() == "else" {
		p.word()
		els, err := p.block(inherit || b.ctrlsIgnoreHitPause)
		if err != nil {
			return nil, err
		}
		if els.ignoreHitPause {
			b.ignoreHitPause = true
		}
		b.els = els
	}
	return b, nil
}

func (p *parser) rest() string {
	end := p.pos + 40
	if end > len(p.src) {
		end = len(p.src)
	}
	return p.src[p.pos:end]
}

// attributes parses `ignoreHitPause` and `persistent(N)` (blockAttribSet,
// compiler.go:7795).
func (p *parser) attributes(b *blockStmt) error {
	for {
		switch p.peekWord() {
		case "ignorehitpause":
			if b.ignoreHitPause {
				return fmt.Errorf("ignoreHitPause given twice (or inside an ignoreHitPause block)")
			}
			b.ignoreHitPause, b.ctrlsIgnoreHitPause = true, true
			p.word()
		case "persistent":
			if p.isFunc {
				return fmt.Errorf("persistent cannot be used in a function")
			}
			if p.stateNo < 0 {
				return fmt.Errorf("persistent cannot be used in a negative state")
			}
			if b.persistIdx >= 0 {
				return fmt.Errorf("persistent given twice on one block")
			}
			p.word()
			p.skipSpace()
			if p.pos >= len(p.src) || p.src[p.pos] != '(' {
				return fmt.Errorf("persistent must be followed by (N), found %q", snip(p.rest()))
			}
			end := strings.IndexByte(p.src[p.pos:], ')')
			if end < 0 {
				return fmt.Errorf("persistent( is missing its \")\"")
			}
			arg := strings.TrimSpace(p.src[p.pos+1 : p.pos+end])
			n, err := strconv.ParseInt(arg, 10, 32)
			if err != nil {
				return fmt.Errorf("persistent(%s): not an integer", snip(arg))
			}
			p.pos += end + 1
			b.persistN = int32(n)
			if n <= 0 {
				b.persistN = math.MaxInt32
			}
			if p.cb.numPersistents >= evaluator.MaxPersistent {
				return fmt.Errorf("more than %d persistent blocks in one Statedef", evaluator.MaxPersistent)
			}
			b.persistIdx = p.cb.numPersistents
			p.cb.numPersistents++
		default:
			return nil
		}
	}
}

// let parses `let a, b = e1, e2;` or `let a, b = call F(args);` after the
// "let" word (letAssign, compiler.go:8179). Targets get their slots before
// the expressions are checked, as in Ikemen GO.
func (p *parser) let() (stmt, error) {
	start := p.pos
	eq := -1
	for i := p.pos; i < len(p.src) && eq < 0; i++ {
		switch p.src[i] {
		case '=':
			eq = i
		case ';', '{', '}':
			return nil, fmt.Errorf("\"let %s\" has no \"=\"", snip(p.src[start:i]))
		}
	}
	if eq < 0 {
		return nil, fmt.Errorf("\"let %s\" has no \"=\"", snip(p.src[start:]))
	}
	head := "let " + snip(p.src[start:eq])
	var slots []int
	var seen []string
	for _, n := range splitTop(p.src[start:eq]) {
		n = strings.ToLower(n)
		if err := checkVarName(n); err != nil {
			return nil, fmt.Errorf("in %q: %w", head, err)
		}
		for _, s := range seen {
			if s == n && n != "_" {
				return nil, fmt.Errorf("duplicate name %q in %q", n, head)
			}
		}
		seen = append(seen, n)
		slot, err := p.declare(n)
		if err != nil {
			return nil, err
		}
		slots = append(slots, slot)
	}
	p.pos = eq + 1
	if p.peekWord() == "call" {
		p.word()
		return p.call(slots)
	}
	text, term := p.sentence()
	stmtText := head + " = " + snip(text)
	if term != ';' {
		return nil, fmt.Errorf("%q is missing its \";\"", stmtText)
	}
	exprs := splitTop(text)
	if len(exprs) != len(slots) {
		return nil, fmt.Errorf("%q has %d names but %d expressions", stmtText, len(slots), len(exprs))
	}
	for _, e := range exprs {
		if e == "" {
			return nil, fmt.Errorf("%q has an empty expression", stmtText)
		}
		if err := p.checkLocals(e); err != nil {
			return nil, err
		}
	}
	return letStmt{slots: slots, exprs: exprs}, nil
}

// call parses `F(args);` after the "call" word (callFunc, compiler.go:8105).
func (p *parser) call(rets []int) (stmt, error) {
	text, term := p.sentence()
	if term != ';' {
		return nil, fmt.Errorf("\"call %s\" is missing its \";\"", snip(text))
	}
	open := strings.IndexByte(text, '(')
	if open < 0 || !strings.HasSuffix(text, ")") {
		return nil, fmt.Errorf("\"call %s\" is not of the form call Name(args)", snip(text))
	}
	name := strings.ToLower(strings.TrimSpace(text[:open]))
	if err := checkVarName(name); err != nil {
		return nil, fmt.Errorf("in \"call %s\": %w", snip(text), err)
	}
	inner := text[open+1 : len(text)-1]
	depth := 0
	for i := 0; i < len(inner); i++ {
		switch inner[i] {
		case '"':
			i += strings.IndexByte(inner[i+1:], '"') + 1
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("\"call %s\" has tokens after its \")\"", snip(text))
			}
		}
	}
	var args []string
	if strings.TrimSpace(inner) != "" {
		args = splitTop(inner)
		for _, a := range args {
			if a == "" {
				return nil, fmt.Errorf("\"call %s\" has an empty argument", snip(text))
			}
			if err := p.checkLocals(a); err != nil {
				return nil, err
			}
		}
	}
	return callStmt{name: name, args: args, rets: rets}, nil
}

// simple parses a controller call or an assigning expression statement
// (stateBlock default case, compiler.go:8340-8393).
func (p *parser) simple() (stmt, error) {
	text, term := p.sentence()
	switch {
	case term == '{' && isIdent(text):
		return p.controller(text)
	case term == '{':
		return nil, fmt.Errorf("unexpected \"{\" after %q", snip(text))
	case term != ';':
		return nil, fmt.Errorf("statement %q is missing its \";\"", snip(text))
	}
	if err := p.checkLocals(text); err != nil {
		return nil, err
	}
	if !hasAssign(text) {
		return nil, fmt.Errorf("Expression with unused value: %q", snip(text))
	}
	return exprStmt{expr: text}, nil
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isIdentByte(s[i]) {
			return false
		}
	}
	return true
}

// controller parses the `{key: value; ...}` part of a controller call.
// Values may span lines and hold commas and brackets; keys are lower-cased
// as cns.Controller.Parameters's are.
func (p *parser) controller(name string) (stmt, error) {
	params := map[string]string{}
	for {
		p.skipSpace()
		if p.pos < len(p.src) && p.src[p.pos] == '}' {
			p.pos++
			break
		}
		colon := strings.IndexByte(p.src[p.pos:], ':')
		if colon < 0 {
			return nil, fmt.Errorf("controller %s: missing \"}\"", name)
		}
		key := strings.ToLower(strings.TrimSpace(p.src[p.pos : p.pos+colon]))
		if !isIdent(strings.ReplaceAll(key, ".", "_")) {
			return nil, fmt.Errorf("controller %s: expected \"key: value\", found %q", name, snip(p.src[p.pos:p.pos+colon+1]))
		}
		p.pos += colon + 1
		value, term := p.sentence()
		if value == "" {
			return nil, fmt.Errorf("controller %s: parameter %q has no value", name, key)
		}
		if term != ';' && term != '}' {
			return nil, fmt.Errorf("controller %s: parameter %q is missing its \";\" or \"}\"", name, key)
		}
		if _, dup := params[key]; dup {
			return nil, fmt.Errorf("controller %s: duplicate parameter %q", name, key)
		}
		if err := p.checkLocals(value); err != nil {
			return nil, fmt.Errorf("controller %s: %w", name, err)
		}
		params[key] = value
		if term == '}' {
			break
		}
	}
	return ctrlStmt{ctrl: cns.Controller{Type: name, Parameters: params}}, nil
}

// statedefHeader converts a Statedef header to a cns.StateDef, the fields
// Ikemen GO's stateDef (src/compiler.go:7020-7130, commit 54d4148a) reads.
// shortcut: only literal values convert; a header field written as an
// expression (Ikemen GO evaluates it on state entry) keeps the zero value,
// upgrade when a real character needs it (none of the reference files do).
func statedefHeader(b zss.Block) cns.StateDef {
	h := b.HeaderParams
	return cns.StateDef{
		Number:          b.Number,
		Type:            cns.StateType(h["type"]),
		MoveType:        cns.MoveType(h["movetype"]),
		Physics:         cns.PhysicsType(h["physics"]),
		Anim:            literalInt(h["anim"]),
		Ctrl:            literalInt(h["ctrl"]) != 0,
		PowerAdd:        literalInt(h["poweradd"]),
		Juggle:          literalInt(h["juggle"]),
		FaceP2:          literalInt(h["facep2"]) != 0,
		SprPriority:     literalInt(h["sprpriority"]),
		HitDefPersist:   literalInt(h["hitdefpersist"]) != 0,
		MoveHitPersist:  literalInt(h["movehitpersist"]) != 0,
		HitCountPersist: literalInt(h["hitcountpersist"]) != 0,
	}
}

// literalInt is s as an integer, 0 when it is absent or not a plain literal.
func literalInt(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}
