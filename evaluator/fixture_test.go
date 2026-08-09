package evaluator

import (
	"bufio"
	"os"
	"strings"
	"testing"

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
		AnimTime:       0,
		Ctrl:           true,
		ActiveCommands: map[string]bool{"holdback": true},
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
	}

	evaluated := 0
	for _, trig := range triggers {
		want, known := wantTrue[trig]
		if !known {
			continue // StateType != A is asserted separately below
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

func TestEval_RealUnsupportedTriggerFromCNSFixture_ReturnsDescriptiveError(t *testing.T) {
	triggers := extractTriggerStrings(t, "testdata/kfm_triggers.cns")

	found := false
	for _, trig := range triggers {
		if trig != "StateType != A" {
			continue
		}
		found = true
		if _, err := Evaluate(trig, Context{}); err == nil {
			t.Errorf("Evaluate(%q) expected a descriptive error for an unsupported real trigger name, got nil", trig)
		}
	}
	if !found {
		t.Fatal(`fixture is expected to contain the real, unsupported trigger "StateType != A"`)
	}
}
