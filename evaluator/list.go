package evaluator

import (
	"strings"
	"unicode"
)

// listCache memoizes SplitList by the raw parameter text, which is loaded
// once per character: splitting it again every tick would allocate (the
// parseCache convention). Unsynchronized: single-threaded engine.
var listCache = map[string][]string{}

// SplitList splits a controller parameter written as a list of expressions
// ("damage = floor(90*root,fvar(0)),10") at its top-level commas, as Ikemen
// GO reads one expression after another: a comma inside parentheses or right
// after a redirection ("root,", "enemy(1),") belongs to the expression. Each
// part is trimmed. The result is shared: do not modify it.
func SplitList(raw string) []string {
	if parts, ok := listCache[raw]; ok {
		return parts
	}
	var parts []string
	depth, start := 0, 0
	// opener holds, for each open parenthesis, whether a redirection keyword
	// comes right before it ("enemy(", "helper(").
	var opener []bool
	closedRedirect := false
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '(':
			opener = append(opener, IsRedirect(lastWord(raw[start:i])))
			depth++
			closedRedirect = false
		case ')':
			if depth > 0 {
				depth--
				closedRedirect = opener[len(opener)-1]
				opener = opener[:len(opener)-1]
			}
		case ',':
			if depth > 0 {
				continue
			}
			before := strings.TrimRightFunc(raw[start:i], unicode.IsSpace)
			if IsRedirect(lastWord(before)) || (closedRedirect && strings.HasSuffix(before, ")")) {
				continue
			}
			parts = append(parts, strings.TrimSpace(raw[start:i]))
			start = i + 1
		default:
			if !unicode.IsSpace(rune(raw[i])) {
				closedRedirect = false
			}
		}
	}
	parts = append(parts, strings.TrimSpace(raw[start:]))
	listCache[raw] = parts
	return parts
}

// lastWord is the identifier text ends with, "" when it ends otherwise.
func lastWord(text string) string {
	text = strings.TrimRightFunc(text, unicode.IsSpace)
	i := len(text)
	for i > 0 {
		c := text[i-1]
		if c == '_' || c == '.' || unicode.IsLetter(rune(c)) || unicode.IsDigit(rune(c)) {
			i--
			continue
		}
		break
	}
	return text[i:]
}
