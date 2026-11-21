package evaluator

import (
	"fmt"
	"strings"
)

// Attack attributes as HitDef's attr and the HitDefAttr trigger name them: a
// state type (S, C, A) and an attack class (NA, NT, NP, SA, ST, SP, HA, HT,
// HP). The bit values are the engine's own; only Ikemen GO's rule for matching
// them is shared (char.go:1104 testAttr).
const (
	attrS = 1 << iota
	attrC
	attrA
	attrNA
	attrNT
	attrNP
	attrSA
	attrST
	attrSP
	attrHA
	attrHT
	attrHP

	attrStateTypes = attrS | attrC | attrA
	attrAA         = attrNA | attrSA | attrHA
	attrAT         = attrNT | attrST | attrHT
	attrAP         = attrNP | attrSP | attrHP
)

// IsHyper, IsSpecial and IsThrow report an attr of a hyper (HA, HT, HP),
// special (SA, ST, SP) or throw (NT, ST, HT) attack: Ikemen GO's AT_AH,
// AT_AS and AT_AT masks, which pick a round's win type (char.go:8984-8990).
func IsHyper(attr int32) bool   { return attr&(attrHA|attrHT|attrHP) != 0 }
func IsSpecial(attr int32) bool { return attr&(attrSA|attrST|attrSP) != 0 }
func IsThrow(attr int32) bool   { return attr&attrAT != 0 }

// attackClasses are the second and later words of an attr: Ikemen GO's
// compiler.go:769-828 (attrMask) and 880-895 (trgAttr). A slice searched with
// strings.EqualFold, not a map of lower-cased words, so that reading a HitDef's
// attr on the tick it runs allocates nothing.
var attackClasses = [...]struct {
	word  string
	class int32
}{
	{"na", attrNA}, {"nt", attrNT}, {"np", attrNP},
	{"sa", attrSA}, {"st", attrST}, {"sp", attrSP},
	{"ha", attrHA}, {"ht", attrHT}, {"hp", attrHP},
	{"aa", attrAA}, {"at", attrAT}, {"ap", attrAP},
	{"n", attrNA | attrNT | attrNP},
	{"s", attrSA | attrST | attrSP},
	{"h", attrHA | attrHT | attrHP},
	// Ikemen GO reads "a" like "h" in both places (compiler.go:803, 893).
	{"a", attrHA | attrHT | attrHP},
}

// attackClass is the attack classes the word names, ok false for no class.
func attackClass(word string) (class int32, ok bool) {
	for _, c := range attackClasses {
		if strings.EqualFold(word, c.word) {
			return c.class, true
		}
	}
	return 0, false
}

// ParseAttr reads an attr such as "S, NA" or "SCA, NA, SA": the first word
// holds state types, each later word an attack class. A HitDef's attr
// (hitdef true) takes one state type, the last letter winning (compiler.go:
// 741-759); the HitDefAttr trigger's (hitdef false) takes any of them. Letters
// and words are case-insensitive. It reads them as Ikemen GO reads a CNS
// attr (attrMask without strict, compiler.go:735-826): a non-letter ends the
// state types, a word counts by its first two letters and a longer one ends
// the attr, an unknown word ends it with what was read; a letter that is not
// a state type is an error. It allocates only to report an error.
// shortcut: a .zss attr is read as leniently, where Ikemen GO is strict.
func ParseAttr(text string, hitdef bool) (int32, error) {
	first, rest, _ := strings.Cut(text, ",")
	var attr int32
	for _, c := range strings.TrimSpace(first) {
		var bit int32
		switch c {
		case 'S', 's':
			bit = attrS
		case 'C', 'c':
			bit = attrC
		case 'A', 'a':
			bit = attrA
		default:
			// A non-letter ends the state types (attrMask, compiler.go:760).
			if c < 128 && !isASCIILetter(c) {
				return attr, nil
			}
			return 0, fmt.Errorf("evaluator: invalid attr %q: %q is not a state type (S, C or A)", text, c)
		}
		if hitdef {
			attr &^= attrStateTypes
		}
		attr |= bit
	}
	for rest != "" {
		var word string
		word, rest, _ = strings.Cut(rest, ",")
		word = strings.TrimSpace(word)
		// A CNS attr reads two letters of a word, and stops after a longer
		// one (attrMask, compiler.go:769-826): "NASAHANP" is NA.
		long := len(word) > 2
		if long {
			word = strings.TrimSpace(word[:2])
		}
		class, ok := attackClass(word)
		if !ok {
			return attr, nil // Ikemen GO warns and keeps what it read
		}
		attr |= class
		if long {
			break
		}
	}
	return attr, nil
}

// isASCIILetter reports an A-Z or a-z rune.
func isASCIILetter(c rune) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// attrMatches is Ikemen GO's HitDef.testAttr: the two attrs share a state type
// and an attack class.
func attrMatches(want, have int32) bool {
	both := want & have
	return both&attrStateTypes != 0 && both&^attrStateTypes != 0
}

// evalHitDefAttr is "HitDefAttr = attr" and "HitDefAttr != attr": true while the
// fighter is in an attack move type and its HitDef's attr matches (char.go:5829,
// compiler.go:3030-3057).
//
// ReversalDefAttr (Ikemen GO) tests the armed ReversalDef's attr the same
// way, in any move type (char.go:1109-1112, 6330).
func evalHitDefAttr(n hitDefAttrNode, ctx *Context) Value {
	if n.reversal {
		return boolValue(attrMatches(n.attr, ctx.ReversalAttr) != n.not)
	}
	matches := ctx.MoveType == 'A' && attrMatches(n.attr, ctx.HitDefAttr)
	return boolValue(matches != n.not)
}
