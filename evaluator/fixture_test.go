package evaluator

import (
	"bufio"
	"os"
	"strings"
	"testing"

	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/match"
)

// extractTriggerStrings reads a .cns-formatted fixture and returns every
// "triggerN"/"triggerall" value in file order, using the same "trigger"
// prefix + first-"=" split rule character/cns.Parse uses to build
// Controller.Triggers — so the strings handed to the evaluator here are
// exactly what a real Controller.Triggers slice would contain, not
// expressions invented for this test file.
func extractTriggerStrings(t *testing.T, path string) []string {
	t.Helper()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open fixture %q: %v", path, err)
	}
	defer f.Close()

	var triggers []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "[") {
			continue
		}
		idx := strings.Index(line, "=")
		if idx < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		value := strings.TrimSpace(line[idx+1:])
		if strings.HasPrefix(key, "trigger") {
			triggers = append(triggers, value)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("failed to read fixture %q: %v", path, err)
	}
	return triggers
}

func TestEval_RealTriggerStringsFromCNSFixture_EvaluateToExpectedValues(t *testing.T) {
	triggers := extractTriggerStrings(t, "testdata/kfm_triggers.cns")

	// A plausible mid-match fighter: in state 200, first tick, animation
	// just started, has control, and is currently holding "back".
	ctx := Context{
		FighterState:   match.FighterState{StateNo: 200},
		Time:           0,
		AnimElapsed:    0,
		Ctrl:           true,
		ActiveCommands: map[string]bool{"holdback": true},
		StateType:      'S',
	}

	wantTrue := map[string]bool{
		`Command = "holdback"`:   true,
		`Ctrl`:                   true,
		`Command = "holddown"`:   false,
		`!Time`:                  true,
		`AnimTime = 0`:           true,
		`StateNo = 200`:          true,
		`var(0) = 0`:             true,
		`sysvar(0) != 1`:         true,
		`IfElse(Ctrl, 1, 0) = 1`: true,
		`StateType != A`:         true,
	}

	evaluated := 0
	for _, trig := range triggers {
		want, known := wantTrue[trig]
		if !known {
			continue
		}
		got, err := Evaluate(trig, ctx)
		if err != nil {
			t.Errorf("Evaluate(%q) returned unexpected error: %v", trig, err)
			continue
		}
		if got.Bool() != want {
			t.Errorf("Evaluate(%q) = %v, want %v", trig, got.Bool(), want)
		}
		evaluated++
	}
	if evaluated != len(wantTrue) {
		t.Fatalf("evaluated %d of the %d expected fixture triggers -- fixture extraction likely missed a line", evaluated, len(wantTrue))
	}
}

// TestParse_ReferenceStateFileTriggers_AllParse parses every trigger of the
// reference .cns and .cmd files (MUGEN's common1.cns and Kung Fu Man's), as
// character/cns hands them to the engine: the grammar must accept them all,
// as Ikemen GO does.
func TestParse_ReferenceStateFileTriggers_AllParse(t *testing.T) {
	for _, path := range []string{
		"../testdata/reference/common1.cns",
		"../testdata/reference/chars/kfm/kfm.cns",
		"../testdata/reference/chars/kfm/kfm.cmd",
	} {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		states, err := cns.Parse(f)
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		count := 0
		for _, def := range states {
			for _, ctrl := range def.Controllers {
				for _, trig := range ctrl.Triggers {
					count++
					if _, err := Parse(trig); err != nil {
						t.Errorf("%s, state %d: %v", path, def.Number, err)
					}
				}
			}
		}
		if count == 0 {
			t.Errorf("%s: no trigger found", path)
		}
	}
}
