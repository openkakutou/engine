package conformance

import (
	"testing"

	"github.com/openkakutou/character"
)

const dupOwnCNS = "[Statedef 0]\nanim = 1\n\n[Statedef 5]\nanim = 5\n"
const dupStCNS = "[Statedef 0]\nanim = 7\n\n[Statedef 9]\nanim = 9\n\n[Statedef 9]\nanim = 99\n"
const dupCommonCNS = "[Statedef 0]\nanim = 8\n\n[Statedef 5000]\nanim = 50\n"

func TestBuildStateTable_DuplicateStateNumbers_FirstFileInReferenceOrderWins(t *testing.T) {
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "dup", dupOwnCNS, "cns = dup.cns\nst = dup_st.cns\nstcommon = dup_common.cns",
		map[string]string{"dup_st.cns": dupStCNS, "dup_common.cns": dupCommonCNS})
	ch, err := character.Load(defPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	table, err := buildStateTable(ch, defPath)
	if err != nil {
		t.Fatalf("buildStateTable: %v", err)
	}
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

func TestBuildStateTable_MissingStcommonFile_IsIgnored(t *testing.T) {
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "nocommon", dupOwnCNS, "cns = nocommon.cns\nstcommon = common1.cns", nil)
	ch, err := character.Load(defPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	table, err := buildStateTable(ch, defPath)
	if err != nil {
		t.Fatalf("buildStateTable: %v", err)
	}
	if len(table) != 2 {
		t.Errorf("table has %d states, want 2 (the cns file's)", len(table))
	}
}

func TestBuildStateTable_UnparsableStateFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "bad", dupOwnCNS, "cns = bad.cns\nst = bad_st.cns", map[string]string{"bad_st.cns": "[Statedef abc]\n"})
	ch, err := character.Load(defPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if _, err := buildStateTable(ch, defPath); err == nil {
		t.Fatal("buildStateTable returned nil error for an unparsable state file")
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

func TestBuildStateTable_CmdFileWithoutStatedefMinusOne_HasState(t *testing.T) {
	dir := t.TempDir()
	defPath := writeCharacter(t, dir, "nohdr", dupOwnCNS, "cns = nohdr.cns\ncmd = nohdr.cmd",
		map[string]string{"nohdr.cmd": cmdWithoutStatedefCMD})
	ch, err := character.Load(defPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	table, err := buildStateTable(ch, defPath)
	if err != nil {
		t.Fatalf("buildStateTable: %v", err)
	}
	def, ok := table[-1]
	if !ok || len(def.Controllers) != 1 || def.Controllers[0].Type != "ChangeState" {
		t.Errorf("state -1 = %+v (present %v), want one ChangeState controller", def, ok)
	}
}
