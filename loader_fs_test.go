package engine

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/openkakutou/character"
	"github.com/openkakutou/engine/v2/match"
)

// mapFSOf copies every file under dir into a MapFS, named relative to dir.
func mapFSOf(t *testing.T, dir string) fstest.MapFS {
	t.Helper()
	fsys := fstest.MapFS{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		fsys[filepath.ToSlash(rel)] = &fstest.MapFile{Data: data}
		return nil
	})
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}
	return fsys
}

// stcommonChar is a character holding only a stcommon entry and its own state 0.
func stcommonChar(entry string) *character.Character {
	return &character.Character{StateCommonFile: entry}
}

func TestLoadFighterProgramFS_KfmInMapFS_BuildsTheProgramOfTheDisk(t *testing.T) {
	defPath := filepath.Join(referenceDir, kfmCnsDef)
	ch := loadChar(t, defPath)
	want, wantMissing, err := LoadFighterProgram(ch, defPath, referenceDir)
	if err != nil {
		t.Fatalf("LoadFighterProgram: %v", err)
	}
	got, gotMissing, err := LoadFighterProgramFS(mapFSOf(t, referenceDir), ch, kfmCnsDef, ".")
	if err != nil {
		t.Fatalf("LoadFighterProgramFS: %v", err)
	}
	if len(got.States) == 0 || len(got.States) != len(want.States) {
		t.Fatalf("states = %d, want %d (non-zero)", len(got.States), len(want.States))
	}
	if _, ok := got.States[5000]; !ok {
		t.Error("state 5000 of common1.cns (a search dir file) is missing")
	}
	if !reflect.DeepEqual(got.States, want.States) {
		t.Error("States differ from the disk program")
	}
	if !reflect.DeepEqual(got.Constants, want.Constants) || got.Constants == nil {
		t.Error("Constants differ from the disk program, or are nil")
	}
	if len(gotMissing) != 0 || len(wantMissing) != 0 {
		t.Errorf("missing = %v (disk %v), want none", gotMissing, wantMissing)
	}
}

func TestLoadFighterProgramFS_KfmWithoutSearchDir_ReportsStcommonMissing(t *testing.T) {
	defPath := filepath.Join(referenceDir, kfmCnsDef)
	ch := loadChar(t, defPath)
	_, missing, err := LoadFighterProgramFS(mapFSOf(t, referenceDir), ch, kfmCnsDef)
	if err != nil {
		t.Fatalf("LoadFighterProgramFS: %v", err)
	}
	if len(missing) != 1 || missing[0] != "common1.cns" {
		t.Errorf("missing = %v, want [common1.cns]", missing)
	}
}

func TestLoadFighterProgramFS_DefFolderAndSearchDirs_DefFolderThenSearchOrderWins(t *testing.T) {
	fsys := fstest.MapFS{
		"chars/a/a.def":       {Data: []byte("")},
		"chars/a/common1.cns": {Data: []byte("[Statedef 0]\nanim = 777\n")},
		"s1/common1.cns":      {Data: []byte("[Statedef 0]\nanim = 111\n\n[Statedef 5000]\nanim = 1\n")},
		"s2/common1.cns":      {Data: []byte("[Statedef 0]\nanim = 222\n\n[Statedef 6000]\nanim = 2\n")},
	}
	prog, _, err := LoadFighterProgramFS(fsys, stcommonChar("common1.cns"), "chars/a/a.def", "s1", "s2")
	if err != nil {
		t.Fatalf("LoadFighterProgramFS: %v", err)
	}
	if got := prog.States[0].Anim; got != 777 {
		t.Errorf("state 0 Anim = %d, want 777 (def folder)", got)
	}
	if _, ok := prog.States[5000]; ok {
		t.Error("state 5000 of a search dir is there, want the def folder's file only")
	}

	delete(fsys, "chars/a/common1.cns")
	prog, _, err = LoadFighterProgramFS(fsys, stcommonChar("common1.cns"), "chars/a/a.def", "s1", "s2")
	if err != nil {
		t.Fatalf("LoadFighterProgramFS: %v", err)
	}
	if got := prog.States[0].Anim; got != 111 {
		t.Errorf("state 0 Anim = %d, want 111 (first search dir)", got)
	}
	if _, ok := prog.States[6000]; ok {
		t.Error("state 6000 of the second search dir is there: files must not merge")
	}
}

func TestLoadFighterProgramFS_EntryWithoutFileButWithZss_LoadsTheZss(t *testing.T) {
	fsys := fstest.MapFS{"data/common1.cns.zss": {Data: []byte("[Statedef 5000; type: S;]\nnull{}\n")}}
	prog, missing, err := LoadFighterProgramFS(fsys, stcommonChar("common1.cns"), "chars/a/a.def", "data")
	if err != nil {
		t.Fatalf("LoadFighterProgramFS: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
	if _, err := prog.NewRuntime(match.FighterState{StateNo: 5000}); err != nil {
		t.Errorf("state 5000 of the fallback .zss is missing: %v", err)
	}
}

func TestLoadFighterProgramFS_MixedCaseNamesAndBackslashes_AreFound(t *testing.T) {
	fsys := fstest.MapFS{"Data/Sub/Common1.CNS": {Data: []byte("[Statedef 5000]\nanim = 1\n")}}
	prog, missing, err := LoadFighterProgramFS(fsys, stcommonChar(`sub\common1.cns`), "chars/a/a.def", `Data`)
	if err != nil {
		t.Fatalf("LoadFighterProgramFS: %v", err)
	}
	if len(missing) != 0 {
		t.Errorf("missing = %v, want none", missing)
	}
	if _, ok := prog.States[5000]; !ok {
		t.Error("state 5000 is missing")
	}
}

func TestLoadFighterProgramFS_EntryEscapingTheRoot_IsReportedMissing(t *testing.T) {
	fsys := fstest.MapFS{"outside.cns": {Data: []byte("[Statedef 5000]\nanim = 1\n")}}
	for _, entry := range []string{"../../../outside.cns", "/outside.cns", `..\..\outside.cns`} {
		_, missing, err := LoadFighterProgramFS(fsys, stcommonChar(entry), "a/a.def", "..", "/")
		if err != nil {
			t.Fatalf("%q: LoadFighterProgramFS: %v", entry, err)
		}
		if len(missing) != 1 || missing[0] != entry {
			t.Errorf("%q: missing = %v, want it reported", entry, missing)
		}
	}
}

func TestLoadFighterProgramFS_ParentEntryInsideTheRoot_IsFound(t *testing.T) {
	fsys := fstest.MapFS{"chars/common1.cns": {Data: []byte("[Statedef 5000]\nanim = 1\n")}}
	prog, missing, err := LoadFighterProgramFS(fsys, stcommonChar("../common1.cns"), "chars/a/a.def")
	if err != nil || len(missing) != 0 {
		t.Fatalf("err = %v, missing = %v, want none", err, missing)
	}
	if _, ok := prog.States[5000]; !ok {
		t.Error("state 5000 is missing")
	}
}

func TestLoadFighterProgramFS_BrokenZssInMapFS_ReturnsErrorNamingIt(t *testing.T) {
	fsys := fstest.MapFS{"data/bad.zss": {Data: []byte("[Statedef 5\nnull{}\n")}}
	ch := &character.Character{StateFiles: []string{"bad.zss"}}
	_, _, err := LoadFighterProgramFS(fsys, ch, "a/a.def", "data")
	if err == nil || !strings.Contains(err.Error(), "bad.zss") {
		t.Errorf("err = %v, want one naming bad.zss", err)
	}
}

func TestLoadFighterProgram_SearchDirOutsideTheDefFolder_IsStillSearched(t *testing.T) {
	root := t.TempDir()
	defPath := filepath.Join(root, "chars", "a", "a.def")
	data := filepath.Join(root, "data")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "common1.cns"), []byte("[Statedef 5000]\nanim = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prog, missing, err := LoadFighterProgram(stcommonChar("common1.cns"), defPath, data)
	if err != nil || len(missing) != 0 {
		t.Fatalf("err = %v, missing = %v, want none", err, missing)
	}
	if _, ok := prog.States[5000]; !ok {
		t.Error("state 5000 of the search dir is missing")
	}
}

func TestLoadFighterProgram_RelativePaths_AreResolvedFromTheWorkingDirectory(t *testing.T) {
	defPath := filepath.Join(referenceDir, kfmCnsDef)
	ch := loadChar(t, defPath)
	prog, missing, err := LoadFighterProgram(ch, defPath, referenceDir)
	if err != nil || len(missing) != 0 {
		t.Fatalf("err = %v, missing = %v, want none", err, missing)
	}
	if _, ok := prog.States[5000]; !ok {
		t.Error("state 5000 of the relative search dir is missing")
	}
}

func TestLoadFighterProgram_SymlinkedWorkingDirectoryAndParentSearchDir_ResolvesAsTheOSDoes(t *testing.T) {
	// The OS resolves ".." from the real working directory, not through the
	// symlink the shell reports in $PWD.
	base := t.TempDir()
	real := filepath.Join(base, "real", "deep")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(base, "link-parent"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "link-parent", "cwd")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	// "../data" from the real cwd is real/data; through the link it would be link-parent/data.
	if err := os.MkdirAll(filepath.Join(base, "real", "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "real", "data", "common1.cns"), []byte("[Statedef 5000]\nanim = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PWD", link)
	t.Chdir(link)
	prog, missing, err := LoadFighterProgram(stcommonChar("common1.cns"), "chars/a/a.def", "../data")
	if err != nil || len(missing) != 0 {
		t.Fatalf("err = %v, missing = %v, want none", err, missing)
	}
	if _, ok := prog.States[5000]; !ok {
		t.Error("state 5000 of ../data is missing")
	}
}

func TestLoadFighterProgram_UnparsableZss_ErrorNamesTheFileAsTheCallerSpelledIt(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("bad.zss", []byte("[Statedef 5\nnull{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ch := &character.Character{StateFiles: []string{"bad.zss"}}
	_, _, err := LoadFighterProgram(ch, "./a.def")
	if err == nil || !strings.Contains(err.Error(), `state script "bad.zss"`) {
		t.Errorf("err = %v, want one naming %q as the caller spelled it", err, "bad.zss")
	}
}
