package conformance

import (
	"strings"
	"testing"

	"github.com/openkakutou/character"
	"github.com/openkakutou/engine/match"
)

const dupOwnCNS = "[Statedef 0]\nanim = 1\n\n[Statedef 5]\nanim = 5\n"
const dupStCNS = "[Statedef 0]\nanim = 7\n\n[Statedef 9]\nanim = 9\n\n[Statedef 9]\nanim = 99\n"
const dupCommonCNS = "[Statedef 0]\nanim = 8\n\n[Statedef 5000]\nanim = 50\n"

func TestBuildProgram_DuplicateStateNumbers_FirstFileInReferenceOrderWins(t *testing.T) {
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "dup", dupOwnCNS, "cns = dup.cns\nst = dup_st.cns\nstcommon = dup_common.cns",
		map[string]string{"dup_st.cns": dupStCNS, "dup_common.cns": dupCommonCNS})
	ch, err := character.Load(defPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	prog, err := buildProgram(ch, defPath)
	if err != nil {
		t.Fatalf("buildProgram: %v", err)
	}
	table := prog.States
	// st file beats stcommon and the cns file for state 0; within one file
	// the first definition of state 9 wins (the second is skipped).
	want := map[int]int{0: 7, 5: 5, 9: 9, 5000: 50}
	if len(table) != len(want) {
		t.Errorf("table has %d states, want %d", len(table), len(want))
	}
	for number, anim := range want {
		if got := table[number].Anim; got != anim {
			t.Errorf("state %d Anim = %d, want %d", number, got, anim)
		}
	}
}

func TestBuildProgram_MissingStcommonFile_IsIgnored(t *testing.T) {
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "nocommon", dupOwnCNS, "cns = nocommon.cns\nstcommon = common1.cns", nil)
	ch, err := character.Load(defPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	prog, err := buildProgram(ch, defPath)
	if err != nil {
		t.Fatalf("buildProgram: %v", err)
	}
	table := prog.States
	if len(table) != 2 {
		t.Errorf("table has %d states, want 2 (the cns file's)", len(table))
	}
}

func TestBuildProgram_UnparsableStateFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "bad", dupOwnCNS, "cns = bad.cns\nst = bad_st.cns", map[string]string{"bad_st.cns": "[Statedef abc]\n"})
	ch, err := character.Load(defPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := buildProgram(ch, defPath); err == nil {
		t.Fatal("buildProgram returned nil error for an unparsable state file")
	}
}

const cmdWithoutStatedefCMD = `[Command]
name = "QCF_a"
command = ~D, DF, F, a
time = 15

[State -1, Fireball]
type = ChangeState
value = 200
trigger1 = Command = "QCF_a"
`

func TestBuildProgram_CmdFileWithoutStatedefMinusOne_HasState(t *testing.T) {
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "nohdr", dupOwnCNS, "cns = nohdr.cns\ncmd = nohdr.cmd",
		map[string]string{"nohdr.cmd": cmdWithoutStatedefCMD})
	ch, err := character.Load(defPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	prog, err := buildProgram(ch, defPath)
	if err != nil {
		t.Fatalf("buildProgram: %v", err)
	}
	table := prog.States
	def, ok := table[-1]
	if !ok || len(def.Controllers) != 1 || def.Controllers[0].Type != "ChangeState" {
		t.Errorf("state -1 = %+v (present %v), want one ChangeState controller", def, ok)
	}
}

const zssFirstSt = "[Statedef 9; type: S;]\nnull{}\n\n[Statedef 30; type: S;]\nnull{}\n"
const cnsAfterZss = "[Statedef 0]\nanim = 1\n\n[Statedef 9]\nanim = 99\n\n[Statedef 7]\nanim = 7\n"

func TestBuildProgram_ZssFileBeforeCnsFile_ZssWinsTheSharedStateNumber(t *testing.T) {
	// Ikemen GO compiles st, stN, cmd, stcommon in that order whatever the
	// format and keeps the first definition (src/compiler.go:8776-8798,
	// :7538-7539), so .cns state 9 loses to the earlier .zss one and 7 stays.
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "mix", cnsAfterZss, "cns = mix.cns\nst = mix_st.zss", map[string]string{"mix_st.zss": zssFirstSt})
	ch, err := character.Load(defPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	prog, err := buildProgram(ch, defPath)
	if err != nil {
		t.Fatalf("buildProgram: %v", err)
	}
	if _, ok := prog.States[9]; ok {
		t.Error(".cns state 9 is in the table, want the earlier .zss one to win")
	}
	if prog.States[7].Anim != 7 || prog.States[0].Anim != 1 {
		t.Errorf("states 7, 0 Anim = %d, %d, want 7, 1 (the .cns states nobody else defines)", prog.States[7].Anim, prog.States[0].Anim)
	}
	rt, err := prog.NewRuntime(match.FighterState{StateNo: 30})
	if err != nil || rt.Context.StateNo != 30 {
		t.Errorf("NewRuntime(30) = %+v, %v, want the .zss state 30 to exist", rt.Context.StateNo, err)
	}
}

func TestBuildProgram_StcommonFallsBackToItsZssFile(t *testing.T) {
	// A "stcommon = common1.cns" with no such file but a "common1.cns.zss"
	// next to the character is loaded as .zss (Ikemen GO stateCompile, :7181).
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "fb", dupOwnCNS, "cns = fb.cns\nstcommon = common1.cns", map[string]string{"common1.cns.zss": "[Statedef 5000; type: S;]\nnull{}\n"})
	ch, err := character.Load(defPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	prog, err := buildProgram(ch, defPath)
	if err != nil {
		t.Fatalf("buildProgram: %v", err)
	}
	if _, err := prog.NewRuntime(match.FighterState{StateNo: 5000}); err != nil {
		t.Errorf("state 5000 of the fallback .zss is missing: %v", err)
	}
}

func TestBuildProgram_UnreadableZssFile_ReturnsErrorNamingIt(t *testing.T) {
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "bz", dupOwnCNS, "cns = bz.cns\nst = bz_st.zss", map[string]string{"bz_st.zss": "[Statedef 5\nnull{}\n"})
	ch, err := character.Load(defPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	_, err = buildProgram(ch, defPath)
	if err == nil || !strings.Contains(err.Error(), "bz_st.zss") {
		t.Errorf("err = %v, want one naming bz_st.zss", err)
	}
}

func TestBuildProgram_MissingZssStcommonFile_IsIgnoredLikeAMissingCnsOne(t *testing.T) {
	// kfm_zss.def says "stcommon = common1.cns.zss", a file of Ikemen GO's
	// data/ folder: character.Load records it as unreadable, the harness
	// skips it as it skips a missing common1.cns.
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "nz", dupOwnCNS, "cns = nz.cns\nstcommon = common1.cns.zss", nil)
	ch, err := character.Load(defPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	prog, err := buildProgram(ch, defPath)
	if err != nil {
		t.Fatalf("buildProgram returned %v, want the missing common file ignored", err)
	}
	if len(prog.States) != 2 {
		t.Errorf("table has %d states, want 2 (the cns file's)", len(prog.States))
	}
}
