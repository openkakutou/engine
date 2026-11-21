package evaluator

import (
	"strings"
	"testing"
)

// Ikemen GO: moveContact/moveHit/moveGuarded/moveReversed return |mctime| for
// the matching contact type (char.go:5967-6000); hitfall reads ghv.fallflag
// (bytecode.go:2611) and canRecover is fall_recover && fall_time >=
// fall_recovertime (char.go:5660).

func TestEval_MoveContactFamily_ReadTheContactCounter(t *testing.T) {
	ctx := Context{MoveContactTime: 3}
	for expr, want := range map[string]float64{
		"MoveContact":     3,
		"MoveHit":         3,
		"MoveGuarded":     0,
		"MoveReversed":    0,
		"MoveContact = 3": 1,
		"!MoveHit":        0,
		"MoveHit && Time": 0,
	} {
		if got := evalFloat(t, expr, ctx); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
	for _, expr := range []string{"MoveContact", "MoveHit", "MoveGuarded", "MoveReversed"} {
		if got := evalFloat(t, expr, Context{}); got != 0 {
			t.Errorf("%s with no contact = %v, want 0", expr, got)
		}
	}
}

func TestEval_HitFallAndCanRecover_ReadTheGetHitVars(t *testing.T) {
	cases := []struct {
		name            string
		g               GetHit
		wantFall, wantR float64
	}{
		{"falling, recoverable, time reached", GetHit{Fall: true, FallRecover: true, FallTime: 4, FallRecoverTime: 4}, 1, 1},
		{"falling, recover time not reached", GetHit{Fall: true, FallRecover: true, FallTime: 3, FallRecoverTime: 4}, 1, 0},
		{"falling, recover disabled", GetHit{Fall: true, FallTime: 40, FallRecoverTime: 4}, 1, 0},
		{"not falling", GetHit{}, 0, 0},
	}
	for _, tc := range cases {
		ctx := Context{GetHit: tc.g}
		if got := evalFloat(t, "HitFall", ctx); got != tc.wantFall {
			t.Errorf("%s: HitFall = %v, want %v", tc.name, got, tc.wantFall)
		}
		if got := evalFloat(t, "CanRecover", ctx); got != tc.wantR {
			t.Errorf("%s: CanRecover = %v, want %v", tc.name, got, tc.wantR)
		}
	}
}

func TestEval_GetHitVar_ReadsTheRemainingGetHitFields(t *testing.T) {
	ctx := Context{GetHit: GetHit{
		Active: true, Yvel: -6.5, Yaccel: 0.35, Fall: true, FallYvel: -4.5,
		CtrlTime: 11, FallRecover: true, FallRecoverTime: 4,
	}}
	for expr, want := range map[string]float64{
		"GetHitVar(yvel)":                         -6.5,
		"GetHitVar(yaccel)":                       0.35,
		"GetHitVar(fall)":                         1,
		"GetHitVar(fall.yvel)":                    -4.5,
		"GetHitVar(ctrltime)":                     11,
		"GetHitVar(fall.recover)":                 1,
		"GetHitVar(fall.recovertime)":             4,
		"GetHitVar(isbound)":                      0, // no binds yet: never bound
		"GetHitVar(yvel) != 0 || GetHitVar(fall)": 1,
	} {
		if got := evalFloat(t, expr, ctx); got != want {
			t.Errorf("%s = %v, want %v", expr, got, want)
		}
	}
	e, err := Parse("GetHitVar(nosuchvar)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Eval(ctx); err == nil || !strings.Contains(err.Error(), "nosuchvar") {
		t.Errorf("GetHitVar(nosuchvar) error = %v, want one naming the variable", err)
	}
}

// ParseAttr/HitDefAttr follow compiler.go attrMask (735-828), trgAttr
// (830-897) and char.go testAttr (1104).

func TestParseAttr_ReadsStateTypesAndAttackClasses(t *testing.T) {
	cases := []struct {
		text    string
		hitdef  bool
		want    int32
		wantErr bool
	}{
		{"S, NA", true, attrS | attrNA, false},
		{"A, HT", true, attrA | attrHT, false},
		{"SCA, NA, SA", false, attrS | attrC | attrA | attrNA | attrSA, false},
		{"S, AT", false, attrS | attrNT | attrST | attrHT, false},
		{"C, N", false, attrC | attrNA | attrNT | attrNP, false},
		{"S, H", false, attrS | attrHA | attrHT | attrHP, false},
		{"s, na", true, attrS | attrNA, false},
		{"SC, NA", true, attrC | attrNA, false}, // a HitDef holds one state type: the last wins (compiler.go:741-759)
		{"X, NA", true, 0, true},
		{"S, ZZ", true, attrS, false}, // an unknown word ends the attr (attrMask, non-strict)
	}
	for _, tc := range cases {
		got, err := ParseAttr(tc.text, tc.hitdef)
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Errorf("ParseAttr(%q, hitdef=%v) = %#x, %v, want %#x, error %v", tc.text, tc.hitdef, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestEval_HitDefAttr_MatchesWhenTheFighterAttacksWithAnOverlappingAttribute(t *testing.T) {
	standingNormal, err := ParseAttr("S, NA", true)
	if err != nil {
		t.Fatal(err)
	}
	ctx := Context{MoveType: 'A', HitDefAttr: standingNormal}
	cases := []struct {
		expr string
		want float64
	}{
		{"HitDefAttr = SCA, NA, SA", 1},
		{"HitDefAttr = SCA, NA", 1},
		{"HitDefAttr = S, NA", 1},
		{"HitDefAttr = C, NA", 0},
		{"HitDefAttr = SCA, HA", 0},
		{"HitDefAttr = S", 0}, // a state type alone has no attack class
		{"HitDefAttr != SCA, SA", 1},
		{"HitDefAttr != SCA, NA", 0},
		{"HitDefAttr = SC,AT", 0},
		{"HitDefAttr = \"S, NA\"", 1},
		{"HitDefAttr = SCA, NA && Life = 5", 0},
		{"p2,hitDefAttr = SCA, NA", 0}, // the opponent's own (zero here)
	}
	for _, tc := range cases {
		if got := evalFloat(t, tc.expr, ctx); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.expr, got, tc.want)
		}
	}
	ctx.MoveType = 'I'
	if got := evalFloat(t, "HitDefAttr = SCA, NA", ctx); got != 0 {
		t.Errorf("HitDefAttr outside an attack move type = %v, want 0 (char.go:5829)", got)
	}
}

func TestEval_HitDefAttr_OfTheOpponent_ReadsItsAttribute(t *testing.T) {
	scAT, err := ParseAttr("SC, AT", false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := Context{P2: Opponent{Valid: true, MoveType: 'A', HitDefAttr: scAT}}
	if got := evalFloat(t, "p2,hitDefAttr = SC,AT", ctx); got != 1 {
		t.Errorf("p2,hitDefAttr = SC,AT = %v, want 1", got)
	}
	if got := evalFloat(t, "p2,hitDefAttr != SC,AT || Life = 5", ctx); got != 0 {
		t.Errorf("p2,hitDefAttr != SC,AT || Life = 5 = %v, want 0", got)
	}
}

func TestParse_HitDefAttrWithABadAttribute_IsAnError(t *testing.T) {
	for _, expr := range []string{"HitDefAttr = XYZ, NA", "HitDefAttr = SCA, QQ", "HitDefAttr = ", "HitDefAttr"} {
		e, err := Parse(expr)
		if err == nil {
			// bare HitDefAttr parses (Ikemen GO pushes the attr string) but cannot be evaluated here
			if _, err = e.Eval(Context{}); err == nil {
				t.Errorf("%q parsed and evaluated, want an error", expr)
			}
		}
	}
}

// attrMask without strict (compiler.go:769-826): Ronald's
// "SCA,NASAHANPSPHPNTSTHT" is SCA, NA; a non-letter ends the state types.
func TestParseAttr_ReadsACNSAttrAsIkemenGO(t *testing.T) {
	for text, want := range map[string]int32{
		"SCA,NASAHANPSPHPNTSTHT": attrStateTypes | attrNA,
		"S1, NA":                 attrS, // the non-letter ends the whole attr
		"SC, NA, XX, SA":         attrS | attrC | attrNA,
	} {
		got, err := ParseAttr(text, false)
		if err != nil || got != want {
			t.Errorf("%q = %#x, %v; want %#x", text, got, err, want)
		}
	}
}
