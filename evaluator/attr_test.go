package evaluator

import "testing"

func TestAttrKinds_MatchIkemenGOsWinTypeMasks(t *testing.T) {
	for text, want := range map[string][3]bool{"S, HA": {true, false, false}, "S, SP": {false, true, false}, "S, NT": {false, false, true}, "S, ST": {false, true, true}, "S, NA": {}} {
		a, err := ParseAttr(text, true)
		if err != nil {
			t.Fatal(err)
		}
		if got := [3]bool{IsHyper(a), IsSpecial(a), IsThrow(a)}; got != want {
			t.Errorf("%s: hyper/special/throw = %v, want %v", text, got, want)
		}
	}
}
