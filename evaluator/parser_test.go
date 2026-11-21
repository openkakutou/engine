package evaluator

import "testing"

// evalOK is a small helper: parse expr and evaluate it against an empty
// Context, failing the test immediately on any error.
func evalOK(t *testing.T, expr string) Value {
	t.Helper()
	e, err := Parse(expr)
	if err != nil {
		t.Fatalf("Parse(%q) returned error: %v", expr, err)
	}
	v, err := e.Eval(Context{})
	if err != nil {
		t.Fatalf("Eval(%q) returned error: %v", expr, err)
	}
	return v
}

func TestParse_MultiplicationBindsTighterThanAddition(t *testing.T) {
	// 1 + 2 * 3 must parse as 1 + (2 * 3) = 7, not (1 + 2) * 3 = 9.
	got := evalOK(t, "1 + 2 * 3")
	if got.Number != 7 {
		t.Errorf("1 + 2 * 3 = %v, want 7", got.Number)
	}
}

func TestParse_LogicalAndBindsTighterThanLogicalOr(t *testing.T) {
	// 1 || 0 && 0 must parse as 1 || (0 && 0) = true, not (1 || 0) && 0 = false.
	got := evalOK(t, "1 || 0 && 0")
	if !got.Bool() {
		t.Errorf("1 || 0 && 0 = %v, want true", got.Bool())
	}
}

func TestParse_UnaryNotBindsTighterThanComparison(t *testing.T) {
	// !1 = 0 must parse as (!1) = 0 -> false = 0 -> true.
	got := evalOK(t, "!1 = 0")
	if !got.Bool() {
		t.Errorf("!1 = 0 = %v, want true", got.Bool())
	}
}

func TestParse_ParenthesesOverridePrecedence(t *testing.T) {
	got := evalOK(t, "(1 + 2) * 3")
	if got.Number != 9 {
		t.Errorf("(1 + 2) * 3 = %v, want 9", got.Number)
	}
}

func TestParse_ReturnsError_OnUnbalancedParentheses(t *testing.T) {
	if _, err := Parse("(1 + 2"); err == nil {
		t.Error("expected error for unbalanced parentheses, got nil")
	}
}

func TestParse_ReturnsError_OnTrailingOperator(t *testing.T) {
	if _, err := Parse("1 +"); err == nil {
		t.Error("expected error for trailing operator, got nil")
	}
}

func TestParse_ReturnsError_OnEmptyExpression(t *testing.T) {
	if _, err := Parse(""); err == nil {
		t.Error("expected error for empty expression, got nil")
	}
}

func TestParse_ReturnsError_OnAdjacentLiteralsWithNoOperator(t *testing.T) {
	if _, err := Parse("1 2"); err == nil {
		t.Error("expected error for adjacent literals with no operator, got nil")
	}
}

func TestParse_ReturnsError_OnInvalidCharacter(t *testing.T) {
	if _, err := Parse("1 @ 2"); err == nil {
		t.Error("expected error for invalid character, got nil")
	}
}

func TestParse_DoesNotPanic_OnDeeplyMalformedInput(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Parse panicked: %v", r)
		}
	}()
	inputs := []string{"(", ")", "((", "&&", "=", "1 = ", "IfElse(", "var(", ","}
	for _, in := range inputs {
		_, _ = Parse(in)
	}
}

// Ikemen GO reads "movetype = H" as one value, so "!" negates the whole
// comparison (Blanka SFA: "!MoveType = H").
func TestEvaluate_NotATypeComparisonNegatesIt(t *testing.T) {
	ctx := &Context{}
	ctx.MoveType = 'H'
	for expr, want := range map[string]bool{"!MoveType = H": false, "!MoveType = A": true, "!movetype != H": true, "!StateType = S": true} {
		v, err := Evaluate(expr, *ctx)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if v.Bool() != want {
			t.Errorf("%s = %v, want %v", expr, v.Bool(), want)
		}
	}
}

// checkEquality with ignoreMostErrors (compiler.go:951-971): in a CNS
// trigger, an old-style trigger's ">=", "<=" or "!=" is read as "=".
func TestEvaluate_OldStyleTriggersTakeAnyEqualityInCNS(t *testing.T) {
	ctx := &Context{}
	ctx.Time = 10
	for expr, want := range map[string]bool{"TimeMod >= 5, 0": true, "TimeMod <= 3, 1": true, "TimeMod != 5, 0": true, "TimeMod >= 3, 0": false} {
		v, err := Evaluate(expr, *ctx)
		if err != nil {
			t.Fatalf("%s: %v", expr, err)
		}
		if v.Bool() != want {
			t.Errorf("%s = %v, want %v", expr, v.Bool(), want)
		}
	}
}
