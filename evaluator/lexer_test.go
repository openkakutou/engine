package evaluator

import "testing"

func tokenKinds(toks []token) []tokenKind {
	kinds := make([]tokenKind, len(toks))
	for i, t := range toks {
		kinds[i] = t.kind
	}
	return kinds
}

func TestLex_TokenizesArithmeticAndComparisonExpression(t *testing.T) {
	toks, err := lex(`1 + 2 * 3 = 7`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []struct {
		kind tokenKind
		text string
	}{
		{tokNumber, "1"},
		{tokOp, "+"},
		{tokNumber, "2"},
		{tokOp, "*"},
		{tokNumber, "3"},
		{tokOp, "="},
		{tokNumber, "7"},
		{tokEOF, ""},
	}

	if len(toks) != len(want) {
		t.Fatalf("got %d tokens, want %d: %+v", len(toks), len(want), toks)
	}
	for i, w := range want {
		if toks[i].kind != w.kind || toks[i].text != w.text {
			t.Errorf("token %d = {%v %q}, want {%v %q}", i, toks[i].kind, toks[i].text, w.kind, w.text)
		}
	}
}

func TestLex_TokenizesStringLiteral_StrippingQuotes(t *testing.T) {
	toks, err := lex(`Command = "holdback"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toks) != 4 { // ident, op, string, EOF
		t.Fatalf("got %d tokens, want 4: %+v", len(toks), toks)
	}
	if toks[0].kind != tokIdent || toks[0].text != "Command" {
		t.Errorf("token 0 = %+v, want ident Command", toks[0])
	}
	if toks[2].kind != tokString || toks[2].text != "holdback" {
		t.Errorf("token 2 = %+v, want string holdback (quotes stripped)", toks[2])
	}
}

func TestLex_TokenizesMultiCharOperators_AsSingleTokens(t *testing.T) {
	toks, err := lex(`a != b <= c >= d && e || f`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var ops []string
	for _, tok := range toks {
		if tok.kind == tokOp {
			ops = append(ops, tok.text)
		}
	}

	wantOps := []string{"!=", "<=", ">=", "&&", "||"}
	if len(ops) != len(wantOps) {
		t.Fatalf("got operators %v, want %v", ops, wantOps)
	}
	for i, w := range wantOps {
		if ops[i] != w {
			t.Errorf("operator %d = %q, want %q", i, ops[i], w)
		}
	}
}

func TestLex_SkipsWhitespaceBetweenTokens(t *testing.T) {
	toks, err := lex("  Ctrl\t&&\n!Time  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Ctrl, &&, !, Time, EOF
	if len(toks) != 5 {
		t.Fatalf("got %d tokens, want 5: %+v", len(toks), toks)
	}
}

func TestLex_ReturnsError_OnUnterminatedStringLiteral(t *testing.T) {
	_, err := lex(`Command = "holdback`)
	if err == nil {
		t.Fatal("expected error for unterminated string literal, got nil")
	}
}

func TestLex_ReturnsError_OnUnexpectedCharacter(t *testing.T) {
	_, err := lex(`a @ b`)
	if err == nil {
		t.Fatal("expected error for unexpected character '@', got nil")
	}
}
