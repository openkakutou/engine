package conformance

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func findCharacter(t *testing.T, r Report, path string) CharacterReport {
	t.Helper()
	for _, c := range r.Characters {
		if c.Path == path {
			return c
		}
	}
	t.Fatalf("no character report for %q in %+v", path, r.Characters)
	return CharacterReport{}
}

func TestRun_CharacterWithUnknownTriggerAndUnsupportedController_ReportsExpectedExposure(t *testing.T) {
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "exposed", exposureCNS, "cns = exposed.cns", nil)

	report, err := Run([]string{defPath}, Options{Ticks: 20})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCharacter(t, report, "exposed.def")

	if c.Status != StatusRan || c.Error != "" {
		t.Fatalf("status = %q error = %q, want ran without error", c.Status, c.Error)
	}
	if c.TicksRun != 20 {
		t.Errorf("TicksRun = %d, want 20", c.TicksRun)
	}
	// 20 ticks x 2 fighters in state 0, once per name per state per tick.
	wantUnsupported := []Count{{Name: "notarealcontroller", Count: 40}}
	if !reflect.DeepEqual(c.UnsupportedControllers, wantUnsupported) {
		t.Errorf("UnsupportedControllers = %+v, want %+v", c.UnsupportedControllers, wantUnsupported)
	}
	// Equal counts are ordered by name ascending; a call is named "name()".
	wantTriggers := []Count{{Name: "bogus2()", Count: 40}, {Name: "foobartrigger", Count: 40}}
	if !reflect.DeepEqual(c.UnknownTriggers, wantTriggers) {
		t.Errorf("UnknownTriggers = %+v, want %+v", c.UnknownTriggers, wantTriggers)
	}
	if c.ControllersUnparsable != 1 {
		t.Errorf("ControllersUnparsable = %d, want 1", c.ControllersUnparsable)
	}
	if c.StatesReached != 1 || c.StatesTotal != 2 {
		t.Errorf("states reached/total = %d/%d, want 1/2", c.StatesReached, c.StatesTotal)
	}
	// State 0 has 3 controllers, 2 of a supported type (VarSet).
	if c.ControllersSupported != 2 || c.ControllersTotal != 3 {
		t.Errorf("controllers supported/total = %d/%d, want 2/3", c.ControllersSupported, c.ControllersTotal)
	}
}

func TestRun_TwoCharacters_AggregatesExposureAndCharacterCount(t *testing.T) {
	root := t.TempDir()
	writeCharacter(t, filepath.Join(root, "one"), "one", exposureCNS, "cns = one.cns", nil)
	writeCharacter(t, filepath.Join(root, "two"), "two", exposureCNS, "cns = two.cns", nil)

	report, err := Run([]string{root}, Options{Ticks: 10})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []AggregateCount{
		{Name: "bogus2()", Exposure: 40, Characters: 2},
		{Name: "foobartrigger", Exposure: 40, Characters: 2},
	}
	if !reflect.DeepEqual(report.UnknownTriggers, want) {
		t.Errorf("aggregate UnknownTriggers = %+v, want %+v", report.UnknownTriggers, want)
	}
	wantTypes := []AggregateCount{{Name: "notarealcontroller", Exposure: 40, Characters: 2}}
	if !reflect.DeepEqual(report.UnsupportedControllers, wantTypes) {
		t.Errorf("aggregate UnsupportedControllers = %+v, want %+v", report.UnsupportedControllers, wantTypes)
	}
}

func TestRun_SameInputTwice_ProducesByteIdenticalOutput(t *testing.T) {
	root := t.TempDir()
	writeCharacter(t, filepath.Join(root, "a"), "a", exposureCNS, "cns = a.cns", nil)
	writeCharacter(t, filepath.Join(root, "b"), "b", tickErrorCNS, "cns = b.cns", nil)

	render := func() (string, string) {
		report, err := Run([]string{root}, Options{Ticks: 30})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		var text, js bytes.Buffer
		if err := report.WriteText(&text); err != nil {
			t.Fatalf("WriteText: %v", err)
		}
		if err := report.WriteJSON(&js); err != nil {
			t.Fatalf("WriteJSON: %v", err)
		}
		return text.String(), js.String()
	}
	text1, json1 := render()
	text2, json2 := render()
	if text1 != text2 || json1 != json2 {
		t.Errorf("two runs differ:\n%s\n---\n%s", text1, text2)
	}
	if text1 == "" || json1 == "" {
		t.Error("empty output")
	}
}

func TestRun_FolderWithNonCharacterDef_SkipsItWithReason(t *testing.T) {
	root := t.TempDir()
	writeCharacter(t, filepath.Join(root, "real"), "real", exposureCNS, "cns = real.cns", nil)
	if err := os.WriteFile(filepath.Join(root, "storyboard.def"), []byte("[SceneDef]\nspr = x.sff\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Run([]string{root}, Options{Ticks: 5})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	skipped := findCharacter(t, report, "storyboard.def")
	if skipped.Status != StatusSkipped || skipped.Error == "" {
		t.Errorf("storyboard.def: status %q error %q, want skipped with a reason", skipped.Status, skipped.Error)
	}
	if got := findCharacter(t, report, filepath.Join("real", "real.def")); got.Status != StatusRan {
		t.Errorf("real.def status = %q, want ran", got.Status)
	}
}

func TestRun_ZssOnlyCharacter_RunsAndCountsItsStatedefs(t *testing.T) {
	dir := t.TempDir()
	zss := "[Statedef 0; type: S; physics: S]\nvarSet{v: 0; value: 1}\n\n[Statedef 200; type: S]\nvarSet{v: 0; value: 2}\n"
	defPath := writeCharacter(t, dir, "zonly", "", "st = zonly.zss", map[string]string{"zonly.zss": zss})

	report, err := Run([]string{defPath}, Options{Ticks: 10})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCharacter(t, report, "zonly.def")
	if c.Status != StatusRan || c.TicksRun != 10 || c.Error != "" {
		t.Errorf("Status, TicksRun, Error = %q, %d, %q, want ran, 10 and none", c.Status, c.TicksRun, c.Error)
	}
	if c.ZssStatedefBlocks != 2 || c.StatesTotal != 2 || c.StatesReached != 1 {
		t.Errorf("ZssStatedefBlocks, StatesTotal, StatesReached = %d, %d, %d, want 2, 2, 1", c.ZssStatedefBlocks, c.StatesTotal, c.StatesReached)
	}
}

func TestRun_ZssOnlyCharacterWithoutStateZero_IsALoadError(t *testing.T) {
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "nozero", "", "st = nozero.zss", map[string]string{"nozero.zss": "[Statedef 200; type: S]\nnull{}\n"})
	report, err := Run([]string{defPath}, Options{Ticks: 10})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if c := findCharacter(t, report, "nozero.def"); c.Status != StatusLoadError || c.Error == "" {
		t.Errorf("Status, Error = %q, %q, want a load error naming the missing state 0", c.Status, c.Error)
	}
}

func TestRun_ZssCharacterThatStopsTheEngine_IsATickErrorNamingTheBlock(t *testing.T) {
	// .zss states are not neutralized: a block the engine cannot run (here an
	// unknown trigger) is the tick error, as for any other engine failure.
	dir := t.TempDir()
	zss := "[Statedef 0; type: S; physics: S]\nif nosuchtrigger {\n null{}\n}\n"
	defPath := writeCharacter(t, dir, "zbad", "", "st = zbad.zss", map[string]string{"zbad.zss": zss})
	report, err := Run([]string{defPath}, Options{Ticks: 10})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCharacter(t, report, "zbad.def")
	if c.Status != StatusTickError || !strings.Contains(c.Error, "state 0") {
		t.Errorf("Status, Error = %q, %q, want a tick error naming state 0", c.Status, c.Error)
	}
}

func TestRun_CnsCharacterWithZssStatedefs_RunsThemAndCountsThem(t *testing.T) {
	dir := t.TempDir()
	zss := "[Statedef 300; type: S]\nnull{}\n"
	defPath := writeCharacter(t, dir, "mixed", exposureCNS, "cns = mixed.cns\nst = mixed.zss", map[string]string{"mixed.zss": zss})

	report, err := Run([]string{defPath}, Options{Ticks: 5})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCharacter(t, report, "mixed.def")
	if c.Status != StatusRan || c.ZssStatedefBlocks != 1 {
		t.Errorf("Status = %q, ZssStatedefBlocks = %d, want ran and 1", c.Status, c.ZssStatedefBlocks)
	}
}

func TestRun_MissingPath_ReturnsError(t *testing.T) {
	_, err := Run([]string{filepath.Join(t.TempDir(), "nope.def")}, Options{Ticks: 5})
	if err == nil {
		t.Fatal("Run returned nil error for a missing path")
	}
}

func TestRun_TickError_IsRecordedAndNextCharacterStillRuns(t *testing.T) {
	root := t.TempDir()
	writeCharacter(t, filepath.Join(root, "a-broken"), "broken", tickErrorCNS, "cns = broken.cns", nil)
	writeCharacter(t, filepath.Join(root, "b-fine"), "fine", exposureCNS, "cns = fine.cns", nil)

	report, err := Run([]string{root}, Options{Ticks: 30})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	broken := findCharacter(t, report, filepath.Join("a-broken", "broken.def"))
	if broken.Status != StatusTickError {
		t.Fatalf("broken Status = %q, want %q", broken.Status, StatusTickError)
	}
	if broken.TicksRun <= 0 || broken.TicksRun >= 30 {
		t.Errorf("broken TicksRun = %d, want between 1 and 29", broken.TicksRun)
	}
	if !bytes.Contains([]byte(broken.Error), []byte("999")) {
		t.Errorf("broken Error = %q, want it to name state 999", broken.Error)
	}
	fine := findCharacter(t, report, filepath.Join("b-fine", "fine.def"))
	if fine.Status != StatusRan || fine.TicksRun != 30 {
		t.Errorf("fine Status = %q TicksRun = %d, want ran and 30", fine.Status, fine.TicksRun)
	}
}

// duplicateCommandCMD declares two commands sharing one name with different
// lengths, which crashes the engine's command matcher (engine bug, tracked
// separately); the harness must record the crash, not die with it.
const duplicateCommandCMD = `[Command]
name = "x"
command = ~D, F, x
time = 30

[Command]
name = "x"
command = ~D, DF, F, x
time = 30
`

func TestRun_EnginePanic_IsRecordedAsTickErrorAndNextCharacterStillRuns(t *testing.T) {
	root := t.TempDir()
	writeCharacter(t, filepath.Join(root, "a-crash"), "crash", exposureCNS, "cns = crash.cns\ncmd = crash.cmd", map[string]string{"crash.cmd": duplicateCommandCMD})
	writeCharacter(t, filepath.Join(root, "b-fine"), "fine", exposureCNS, "cns = fine.cns", nil)

	report, err := Run([]string{root}, Options{Ticks: 600})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	crash := findCharacter(t, report, filepath.Join("a-crash", "crash.def"))
	if crash.Status != StatusTickError {
		t.Fatalf("crash Status = %q, want %q", crash.Status, StatusTickError)
	}
	if !bytes.Contains([]byte(crash.Error), []byte("panic")) {
		t.Errorf("crash Error = %q, want it to say the engine panicked", crash.Error)
	}
	fine := findCharacter(t, report, filepath.Join("b-fine", "fine.def"))
	if fine.Status != StatusRan || fine.TicksRun != 600 {
		t.Errorf("fine Status = %q TicksRun = %d, want ran and 600", fine.Status, fine.TicksRun)
	}
}

const unknownInPlaySndAndGetHitVarCNS = `[Statedef 0]
type = S
physics = S
ctrl = 1

[State 0, Snd]
type = PlaySnd
trigger1 = 1
value = 0, P2StateNo % 3

[State 0, Hit]
type = ChangeState
trigger1 = GetHitVar(yvel) > 0
value = 0
`

func TestRun_UnknownTriggersInPlaySndHalfAndGetHitVarVariable_AreNeutralizedAndCounted(t *testing.T) {
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "hidden", unknownInPlaySndAndGetHitVarCNS, "cns = hidden.cns", nil)
	report, err := Run([]string{defPath}, Options{Ticks: 5})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCharacter(t, report, "hidden.def")
	if c.Status != StatusRan {
		t.Fatalf("status = %q error = %q, want ran", c.Status, c.Error)
	}
	want := []Count{{Name: "gethitvar(yvel)", Count: 10}, {Name: "p2stateno", Count: 10}}
	if !reflect.DeepEqual(c.UnknownTriggers, want) {
		t.Errorf("UnknownTriggers = %+v, want %+v", c.UnknownTriggers, want)
	}
}

func TestRun_CharacterWithCmdLackingStatedefMinusOne_Runs(t *testing.T) {
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "nohdr", dupOwnCNS, "cns = nohdr.cns\ncmd = nohdr.cmd",
		map[string]string{"nohdr.cmd": cmdWithoutStatedefCMD})
	report, err := Run([]string{defPath}, Options{Ticks: 3})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	c := findCharacter(t, report, "nohdr.def")
	if c.Status != StatusRan {
		t.Errorf("status = %q error = %q, want ran", c.Status, c.Error)
	}
}
