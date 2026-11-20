package engine

import (
	"testing"

	"github.com/openkakutou/engine/v2/match"
)

// common.go:931-945 ParseIkemenVersion.
func TestParseIkemenVersion(t *testing.T) {
	for raw, want := range map[string][3]uint16{"": {}, "1.0": {1, 0, 0}, "0.99.2": {0, 99, 2}, "1.x": {1, 0, 0}, " 2 . 1 ": {2, 1, 0}} {
		if got := parseIkemenVersion(raw); got != want {
			t.Errorf("%q = %v, want %v", raw, got, want)
		}
	}
}

// Neither reference kfm.def gives an ikemenversion (both say mugenversion =
// 1.0): Ikemen GO treats them as MUGEN characters.
func TestLoad_TheReferenceCharactersAreMugenCharacters(t *testing.T) {
	for _, def := range []string{kfmZssDef, kfmCnsDef} {
		prog := loadReferenceProgram(t, def)
		if prog.IkemenVersion != ([3]uint16{}) {
			t.Errorf("%s ikemenversion = %v, want unset", def, prog.IkemenVersion)
		}
		rt, err := prog.NewRuntime(match.FighterState{Health: 1000})
		if err != nil {
			t.Fatal(err)
		}
		if rt.Context.IkemenChar {
			t.Errorf("%s: IkemenChar true, want false", def)
		}
	}
}

func TestNewRuntime_AnIkemenVersionMakesAnIkemenCharacter(t *testing.T) {
	prog := loadReferenceProgram(t, kfmCnsDef)
	prog.IkemenVersion = [3]uint16{1, 0, 0}
	rt, err := prog.NewRuntime(match.FighterState{Health: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if !rt.Context.IkemenChar {
		t.Error("ikemenversion 1.0: want IkemenChar")
	}
}

func TestParseMugenVersion(t *testing.T) {
	for raw, want := range map[string][2]uint16{"1.0": {1, 0}, "1.1": {1, 1}, "": {0, 5}, "2002.04.14": {0, 5}, "1.2": {0, 5}, "x": {0, 5}} {
		if got := parseMugenVersion(raw); got != want {
			t.Errorf("%q = %v, want %v", raw, got, want)
		}
	}
}
