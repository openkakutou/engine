package engine

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/openkakutou/character"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/character/zss"
)

const referenceDir = "testdata/reference"

// checkProvenance reads dir/PROVENANCE.md (rows "| path | sha256 | source |
// licence |") and returns one problem per missing, altered or unlisted file.
func checkProvenance(dir string) ([]string, error) {
	f, err := os.Open(filepath.Join(dir, "PROVENANCE.md"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	listed := map[string]string{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		cells := strings.Split(scanner.Text(), "|")
		if len(cells) < 5 || !strings.HasPrefix(strings.TrimSpace(scanner.Text()), "|") {
			continue
		}
		path := strings.Trim(strings.TrimSpace(cells[1]), "`")
		sum := strings.TrimSpace(cells[2])
		if len(sum) != 64 {
			continue // header and separator rows
		}
		listed[path] = sum
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var problems []string
	for path, want := range listed {
		data, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil {
			problems = append(problems, "missing: "+path)
			continue
		}
		got := sha256.Sum256(data)
		if hex.EncodeToString(got[:]) != want {
			problems = append(problems, "altered: "+path)
		}
	}
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, ok := listed[rel]; !ok && rel != "PROVENANCE.md" {
			problems = append(problems, "unlisted: "+rel)
		}
		return nil
	})
	sort.Strings(problems)
	return problems, err
}

func TestReference_FilesMatchProvenance(t *testing.T) {
	problems, err := checkProvenance(referenceDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("reference fixtures differ from PROVENANCE.md: %v", problems)
	}
}

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func provenanceTable(rows ...string) string {
	return "| path | sha256 | source | licence |\n|---|---|---|---|\n" + strings.Join(rows, "\n") + "\n"
}

func TestCheckProvenance_Consistent_ReportsNothing(t *testing.T) {
	dir := t.TempDir()
	sum := writeTemp(t, dir, "a.txt", "hello")
	writeTemp(t, dir, "PROVENANCE.md", provenanceTable(fmt.Sprintf("| a.txt | %s | s | l |", sum)))
	problems, err := checkProvenance(dir)
	if err != nil || len(problems) != 0 {
		t.Fatalf("problems=%v err=%v, want none", problems, err)
	}
}

func TestCheckProvenance_AlteredByte_ReportsAltered(t *testing.T) {
	dir := t.TempDir()
	sum := writeTemp(t, dir, "a.txt", "hello")
	writeTemp(t, dir, "PROVENANCE.md", provenanceTable(fmt.Sprintf("| a.txt | %s | s | l |", sum)))
	writeTemp(t, dir, "a.txt", "hellp")
	problems, _ := checkProvenance(dir)
	if len(problems) != 1 || problems[0] != "altered: a.txt" {
		t.Fatalf("problems=%v, want [altered: a.txt]", problems)
	}
}

func TestCheckProvenance_MissingFile_ReportsMissing(t *testing.T) {
	dir := t.TempDir()
	sum := writeTemp(t, dir, "a.txt", "hello")
	writeTemp(t, dir, "PROVENANCE.md", provenanceTable(fmt.Sprintf("| a.txt | %s | s | l |", sum)))
	if err := os.Remove(filepath.Join(dir, "a.txt")); err != nil {
		t.Fatal(err)
	}
	problems, _ := checkProvenance(dir)
	if len(problems) != 1 || problems[0] != "missing: a.txt" {
		t.Fatalf("problems=%v, want [missing: a.txt]", problems)
	}
}

func TestCheckProvenance_UnlistedFile_ReportsUnlisted(t *testing.T) {
	dir := t.TempDir()
	writeTemp(t, dir, "PROVENANCE.md", provenanceTable())
	writeTemp(t, dir, "extra.txt", "x")
	problems, _ := checkProvenance(dir)
	if len(problems) != 1 || problems[0] != "unlisted: extra.txt" {
		t.Fatalf("problems=%v, want [unlisted: extra.txt]", problems)
	}
}

func hasState(numbers []int, want ...int) []int {
	have := map[int]bool{}
	for _, n := range numbers {
		have[n] = true
	}
	var absent []int
	for _, w := range want {
		if !have[w] {
			absent = append(absent, w)
		}
	}
	return absent
}

// Counts below come from grepping the "[Statedef" headers of the files.
func TestReference_MugenCommon1_ParsesAllStockStates(t *testing.T) {
	f, err := os.Open(filepath.Join(referenceDir, "common1.cns"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	states, err := cns.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 51 {
		t.Errorf("got %d states, want 51", len(states))
	}
	var numbers []int
	for _, s := range states {
		numbers = append(numbers, s.Number)
	}
	if absent := hasState(numbers, 0, 5000, 5100); len(absent) != 0 {
		t.Errorf("missing states %v", absent)
	}
}

func TestReference_IkemenCommon1Zss_ParsesAllStatedefs(t *testing.T) {
	f, err := os.Open(filepath.Join(referenceDir, "common1.cns.zss"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	script, err := zss.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	var numbers []int
	for _, b := range script.Blocks {
		if b.Kind == zss.BlockKindStatedef {
			numbers = append(numbers, b.Number)
		}
	}
	if len(numbers) != 53 {
		t.Errorf("got %d statedefs, want 53", len(numbers))
	}
	if absent := hasState(numbers, 0, 10, 11, 12); len(absent) != 0 {
		t.Errorf("missing statedefs %v", absent)
	}
}

func TestReference_KungFuMan_LoadsFromDef(t *testing.T) {
	for _, def := range []string{"chars/kfm/kfm.def", "chars/kfm_zss/kfm_zss.def"} {
		t.Run(def, func(t *testing.T) {
			c, err := character.Load(filepath.Join(referenceDir, def))
			if err != nil {
				t.Fatal(err)
			}
			if c.Name != "Kung Fu Man" {
				t.Errorf("Name = %q, want %q", c.Name, "Kung Fu Man")
			}
		})
	}
}
