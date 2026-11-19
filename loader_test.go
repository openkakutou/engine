package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openkakutou/character"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/engine/v2/evaluator"
	"github.com/openkakutou/engine/v2/match"
)

// copyKfm copies the reference kfm folder into a fresh directory, with its
// .def's stcommon line replaced, and returns the .def path.
func copyKfm(t *testing.T, stcommon string) (dir, defPath string) {
	t.Helper()
	src := filepath.Join(referenceDir, "chars", "kfm")
	dir = t.TempDir()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(src, entry.Name()))
		if err != nil {
			t.Fatalf("test setup: %v", err)
		}
		if entry.Name() == "kfm.def" {
			data = []byte(strings.Replace(string(data), "stcommon = common1.cns", "stcommon = "+stcommon, 1))
		}
		if err := os.WriteFile(filepath.Join(dir, entry.Name()), data, 0o644); err != nil {
			t.Fatalf("test setup: %v", err)
		}
	}
	return dir, filepath.Join(dir, "kfm.def")
}

func loadChar(t *testing.T, defPath string) *character.Character {
	t.Helper()
	ch, err := character.Load(defPath)
	if err != nil {
		t.Fatalf("character.Load(%s): %v", defPath, err)
	}
	return ch
}

func TestLoadFighterProgram_KfmWithSearchDir_HasStateZeroAndTheCommonCnsStates(t *testing.T) {
	defPath := filepath.Join(referenceDir, kfmCnsDef)
	prog, missing, err := LoadFighterProgram(loadChar(t, defPath), defPath, referenceDir)
	if err != nil {
		t.Fatalf("LoadFighterProgram: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
	f, err := os.Open(filepath.Join(referenceDir, "common1.cns"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	common, err := cns.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	for _, def := range common {
		if _, ok := prog.States[def.Number]; !ok {
			t.Errorf("common1.cns state %d is not reachable", def.Number)
		}
	}
	for _, number := range []int{0, 5000} {
		if _, err := prog.NewRuntime(match.FighterState{StateNo: number}); err != nil {
			t.Errorf("state %d: %v", number, err)
		}
	}
}

func TestLoadFighterProgram_KfmZssWithSearchDir_HasStateZeroFromTheZssCommon(t *testing.T) {
	defPath := filepath.Join(referenceDir, kfmZssDef)
	prog, missing, err := LoadFighterProgram(loadChar(t, defPath), defPath, referenceDir)
	if err != nil {
		t.Fatalf("LoadFighterProgram: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
	if _, ok := prog.States[0]; ok {
		t.Error("state 0 is a .cns state, want it to come from a .zss script")
	}
	for _, number := range []int{0, 5000} {
		if _, err := prog.NewRuntime(match.FighterState{StateNo: number}); err != nil {
			t.Errorf("state %d: %v", number, err)
		}
	}
}

func TestLoadFighterProgram_NoSearchDir_ReportsStcommonMissingAndKeepsOwnStates(t *testing.T) {
	defPath := filepath.Join(referenceDir, kfmCnsDef)
	prog, missing, err := LoadFighterProgram(loadChar(t, defPath), defPath)
	if err != nil {
		t.Fatalf("LoadFighterProgram: %v", err)
	}
	if len(missing) != 1 || missing[0] != "common1.cns" {
		t.Errorf("missing = %v, want [common1.cns]", missing)
	}
	if _, ok := prog.States[-1]; !ok {
		t.Error("kfm's own state -1 (from its .cmd) is missing")
	}
	if _, ok := prog.States[5000]; ok {
		t.Error("state 5000 is there without the common file")
	}
}

func TestLoadFighterProgram_StateFileInCharDirAndSearchDir_CharDirWins(t *testing.T) {
	dir, defPath := copyKfm(t, "common1.cns")
	own := "[Statedef 0]\nanim = 777\n"
	if err := os.WriteFile(filepath.Join(dir, "COMMON1.cns"), []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	prog, missing, err := LoadFighterProgram(loadChar(t, defPath), defPath, referenceDir)
	if err != nil {
		t.Fatalf("LoadFighterProgram: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
	if got := prog.States[0].Anim; got != 777 {
		t.Errorf("state 0 Anim = %d, want 777 (the character folder's file)", got)
	}
	if _, ok := prog.States[5000]; ok {
		t.Error("state 5000 from the search dir's file is there, want the character folder's file only")
	}
}

func TestLoadFighterProgram_BrokenZssInSearchDir_ReturnsErrorNamingIt(t *testing.T) {
	_, defPath := copyKfm(t, "common1.cns.zss")
	search := t.TempDir()
	if err := os.WriteFile(filepath.Join(search, "common1.cns.zss"), []byte("[Statedef 5\nnull{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := LoadFighterProgram(loadChar(t, defPath), defPath, search)
	if err == nil || !strings.Contains(err.Error(), "common1.cns.zss") {
		t.Errorf("err = %v, want one naming common1.cns.zss", err)
	}
}

// kfm.def's "cns" entry (kfm.cns) and kfm_zss.def's (kfm.const) hold the
// character's constants; the fighter's runtime reads them through Const()
// (Ikemen GO src/char.go:4411-4631 reads the .def's "cns" file). Values are
// read off those files: [Movement] yaccel = .44, [Velocity] walk.fwd = 2.4.
func TestLoadFighterProgram_Kfm_ConstantsReachTheRuntimeContext(t *testing.T) {
	for _, defPath := range []string{kfmCnsDef, kfmZssDef} {
		prog := loadReferenceProgram(t, defPath)
		rt, err := prog.NewRuntime(match.FighterState{StateNo: 0, Health: 1000})
		if err != nil {
			t.Fatalf("%s: NewRuntime: %v", defPath, err)
		}
		for expr, want := range map[string]float64{
			"Const(movement.yaccel)":        float64(float32(0.44)),
			"Const(velocity.walk.fwd.x)":    float64(float32(2.4)),
			"Const(data.life)":              1000,
			"Const240p(Const(size.height))": 60,
		} {
			got, err := evaluator.Evaluate(expr, rt.Context)
			if err != nil || got.Number != want {
				t.Errorf("%s: %s = %v, %v, want %v", defPath, expr, got.Number, err, want)
			}
		}
	}
}

// A "cns" entry found in no folder (character.LoadBytes does not read files)
// is reported like a missing state file, and the character keeps Ikemen GO's
// defaults.
func TestLoadFighterProgram_ConstantsFileMissing_IsReportedAndDefaultsApply(t *testing.T) {
	path := filepath.Join(referenceDir, kfmCnsDef)
	ch := loadChar(t, path)
	ch.ConstantsFile = "absent.cns"
	prog, missing, err := LoadFighterProgram(ch, path, referenceDir)
	if err != nil {
		t.Fatalf("LoadFighterProgram: %v", err)
	}
	found := false
	for _, m := range missing {
		found = found || m == "absent.cns"
	}
	if !found {
		t.Errorf("missing = %v, want it to list absent.cns", missing)
	}
	if prog.Constants != nil {
		t.Error("Constants is set although the file is missing; want nil (defaults)")
	}
}

func TestLoadFighterProgram_BrokenConstantsFile_ReturnsErrorNamingIt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.const"), []byte("[Constants]\nbad.one = abc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(referenceDir, kfmCnsDef)
	ch := loadChar(t, path)
	ch.ConstantsFile = "bad.const"
	_, _, err := LoadFighterProgram(ch, path, dir)
	if err == nil || !strings.Contains(err.Error(), "bad.const") || !strings.Contains(err.Error(), "bad.one") {
		t.Errorf("error = %v, want one naming bad.const and bad.one", err)
	}
}

func TestFighterProgram_WithoutConstants_RuntimeReadsIkemenDefaults(t *testing.T) {
	prog := FighterProgram{States: idleStates()}
	rt, err := prog.NewRuntime(match.FighterState{StateNo: 0})
	if err != nil {
		t.Fatal(err)
	}
	got, err := evaluator.Evaluate("Const(movement.yaccel)", rt.Context)
	if err != nil || got.Number != float64(float32(0.44)) {
		t.Errorf("Const(movement.yaccel) = %v, %v, want Ikemen GO's default 0.44 (char.go:539)", got.Number, err)
	}
}
