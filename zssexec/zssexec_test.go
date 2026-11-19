package zssexec

import (
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/evaluator"
	"github.com/openkakutou/engine/match"
)

// mustParseScript builds a CompiledScript from the given block sources, one
// "[Statedef ...]"/"[Function ...]" block plus body per string, joined with
// blank lines the same way real .zss text separates blocks. Compiled (see
// Compile) before returning, since Step now takes a CompiledScript rather
// than a raw zss.Script -- see this item's own ADR.
func mustParseScript(t *testing.T, blocks ...string) CompiledScript {
	t.Helper()
	source := ""
	for i, b := range blocks {
		if i > 0 {
			source += "\n\n"
		}
		source += b
	}
	script, err := zss.Parse(strings.NewReader(source))
	if err != nil {
		t.Fatalf("zss.Parse returned unexpected error: %v", err)
	}
	return Compile(script)
}

func TestStep_IfConditionTrue_AppliesChangeState(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\nif Time = 0 {\n\tchangeState{value: 5;}\n}",
		"[Statedef 5]",
	)
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}, Time: 0}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.StateNo != 5 {
		t.Errorf("StateNo = %d, want 5", result.Context.StateNo)
	}
	if result.Context.Time != 0 {
		t.Errorf("Time = %d, want 0 (must reset on state change)", result.Context.Time)
	}
}

func TestStep_IfConditionFalse_ElseBranchApplied(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\nif Time = 0 {\n\tchangeState{value: 5;}\n} else {\n\tchangeState{value: 9;}\n}",
		"[Statedef 5]",
		"[Statedef 9]",
	)
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}, Time: 3}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.StateNo != 9 {
		t.Errorf("StateNo = %d, want 9 (else branch)", result.Context.StateNo)
	}
}

func TestStep_MultipleIfStatements_StopsAfterChangeStateApplies(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\nif Command = \"holdfwd\" {\n\tchangeState{value: 20;}\n}\nif Command = \"holdback\" {\n\tchangeState{value: 30;}\n}",
		"[Statedef 20]",
		"[Statedef 30]",
	)
	ctx := evaluator.Context{
		FighterState:   match.FighterState{StateNo: 0},
		ActiveCommands: map[string]bool{"holdfwd": true, "holdback": true},
	}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.StateNo != 20 {
		t.Errorf("StateNo = %d, want 20 (first ChangeState wins, second never evaluated)", result.Context.StateNo)
	}
}

func TestStep_ControllerStatement_AssignsVariable(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nvarSet{v: 3; value: 7;}")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.Vars[3] != 7 {
		t.Errorf("Vars[3] = %d, want 7", result.Context.Vars[3])
	}
	if result.Context.StateNo != 0 {
		t.Errorf("StateNo = %d, want 0 (VarSet is not a state change)", result.Context.StateNo)
	}
}

func TestStep_CallFunction_ExecutesFunctionBody(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\ncall MarkEntered();",
		"[Function MarkEntered()]\nvarSet{v: 0; value: 1;}",
	)
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.Vars[0] != 1 {
		t.Errorf("Vars[0] = %d, want 1 (set by the called function)", result.Context.Vars[0])
	}
}

func TestStep_MutuallyRecursiveFunctionCalls_ReturnsDescriptiveErrorInsteadOfCrashing(t *testing.T) {
	// A .zss script is untrusted, mod-authored content: two functions that
	// call each other (or a function that calls itself) must not be able
	// to crash the process via unbounded Go-stack recursion -- Step must
	// return a descriptive error once the call depth is exceeded instead.
	script := mustParseScript(t,
		"[Statedef 0]\ncall A();",
		"[Function A()]\ncall B();",
		"[Function B()]\ncall A();",
	)
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, script)
	if err == nil {
		t.Fatal("expected an error for mutually recursive function calls exceeding the max call depth, got nil")
	}
}

func TestStep_UnsupportedConstruct_ReturnsDescriptiveError(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nfor i = 0, 3 {\n}")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, script)
	if err == nil {
		t.Fatal("expected an error for an unsupported .zss construct (let), got nil")
	}
}

func TestStep_UnknownCurrentState_ReturnsDescriptiveError(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 999}}

	_, err := Step(ctx, script)
	if err == nil {
		t.Fatal("expected an error for a StateNo missing from the loaded script, got nil")
	}
}

func TestStep_ChangeStateTargetingNonexistentState_ReturnsDescriptiveError(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nchangeState{value: 999;}")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, script)
	if err == nil {
		t.Fatal("expected a descriptive error for a ChangeState target missing from the loaded script, got nil")
	}
}

func TestStep_ConditionEvaluationError_ReturnsDescriptiveError(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nif SomeUnknownTrigger {\n\tchangeState{value: 5;}\n}")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	_, err := Step(ctx, script)
	if err == nil {
		t.Fatal("expected an error for an if condition referencing an unknown trigger name, got nil")
	}
}

// TestCompile_DuplicateStatedefNumbers_FirstOccurrenceWins pins Compile's
// precomputed index to the same "first block in declaration order wins"
// semantics the original findStatedef linear scan had -- a naive map-build
// loop that overwrites on every insert would silently flip this to
// last-occurrence-wins instead, with no compiler error to catch it.
func TestCompile_DuplicateStatedefNumbers_FirstOccurrenceWins(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\nvarSet{v: 0; value: 1;}",
		"[Statedef 0]\nvarSet{v: 0; value: 2;}",
	)
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.Vars[0] != 1 {
		t.Errorf("Vars[0] = %d, want 1 (first-declared Statedef 0 block wins, matching the pre-existing linear-scan behavior)", result.Context.Vars[0])
	}
}

// TestCompile_DuplicateFunctionNames_FirstOccurrenceWins is
// TestCompile_DuplicateStatedefNumbers_FirstOccurrenceWins's Function
// counterpart, exercising findFunction's own precomputed index instead.
func TestCompile_DuplicateFunctionNames_FirstOccurrenceWins(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\ncall Helper();",
		"[Function Helper()]\nvarSet{v: 0; value: 1;}",
		"[Function Helper()]\nvarSet{v: 0; value: 2;}",
	)
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.Vars[0] != 1 {
		t.Errorf("Vars[0] = %d, want 1 (first-declared Function Helper() wins, matching the pre-existing linear-scan behavior)", result.Context.Vars[0])
	}
}

func mustParseFixtureScript(t *testing.T, path string) CompiledScript {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	script, err := zss.Parse(f)
	if err != nil {
		t.Fatalf("zss.Parse returned unexpected error: %v", err)
	}
	return Compile(script)
}

// TestStep_RealCharacterFixture_KfmIdle_NoCommandHeld_CallsFunctionAndSetsVar
// loads a .zss idle-state fixture styled after the same Kung Fu Man sample
// character statemachine's own testdata/kfm_idle.cns fixture uses. With no
// movement command held, both ChangeState-guarded ifs are false, so
// execution reaches the Time = 0 branch and calls its helper function,
// confirming a nested function call's controller-style statement mutates
// the fighter's variables.
func TestStep_RealCharacterFixture_KfmIdle_NoCommandHeld_CallsFunctionAndSetsVar(t *testing.T) {
	script := mustParseFixtureScript(t, "testdata/kfm_idle.zss")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}, Time: 0}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.StateNo != 0 {
		t.Errorf("StateNo = %d, want 0 (no command held, no state change)", result.Context.StateNo)
	}
	if result.Context.Vars[0] != 1 {
		t.Errorf("Vars[0] = %d, want 1 (MarkEnteredIdle called since Time = 0)", result.Context.Vars[0])
	}
}

// TestStep_RealCharacterFixture_KfmIdle_HoldFwd_ChangesStateAndStops mirrors
// statemachine's own real-fixture behavior: once the holdfwd command's
// ChangeState applies, the later Time = 0 branch (and its function call)
// never runs for this tick, the same "stop at the first state change" rule
// .cns-driven states follow.
func TestStep_RealCharacterFixture_KfmIdle_HoldFwd_ChangesStateAndStops(t *testing.T) {
	script := mustParseFixtureScript(t, "testdata/kfm_idle.zss")
	ctx := evaluator.Context{
		FighterState:   match.FighterState{StateNo: 0},
		ActiveCommands: map[string]bool{"holdfwd": true},
	}

	result, err := Step(ctx, script)
	if err != nil {
		t.Fatalf("Step returned unexpected error: %v", err)
	}
	if result.Context.StateNo != 20 {
		t.Errorf("StateNo = %d, want 20 (holdfwd command recognized)", result.Context.StateNo)
	}
	if result.Context.Vars[0] != 0 {
		t.Errorf("Vars[0] = %d, want 0 (MarkEnteredIdle never reached once ChangeState applied)", result.Context.Vars[0])
	}
}

// allExist reports every state as existing, as the root package's merged
// state table would for a ChangeState target.
func allExist(int) bool { return true }

func TestStepState_GivenStateNumber_RunsThatStateNotTheCurrentOne(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\nvarSet{v: 1; value: 111}",
		"[Statedef -1]\nvarSet{v: 1; value: 7}",
	)
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := StepState(ctx, -1, script, allExist)
	if err != nil {
		t.Fatalf("StepState returned unexpected error: %v", err)
	}
	if got := result.Context.Vars[1]; got != 7 {
		t.Errorf("var(1) = %d, want 7 (state -1's body, not state 0's 111)", got)
	}
	if result.Context.StateNo != 0 {
		t.Errorf("StateNo = %d, want 0 (a special state does not become the current one)", result.Context.StateNo)
	}
}

func TestStepState_ChangeStateToAStateOnlyTheCallersExistsKnows_Applies(t *testing.T) {
	// Ikemen GO changes state in one table shared by .cns and .zss states
	// (src/char.go changeState looks up sys.cgi[pn].states), so the target
	// may be defined outside this script.
	script := mustParseScript(t, "[Statedef 0]\nchangeState{value: 700}")
	ctx := evaluator.Context{FighterState: match.FighterState{StateNo: 0}}

	result, err := StepState(ctx, 0, script, func(n int) bool { return n == 700 })
	if err != nil {
		t.Fatalf("StepState returned unexpected error: %v", err)
	}
	if result.Context.StateNo != 700 {
		t.Errorf("StateNo = %d, want 700", result.Context.StateNo)
	}
}

func TestStepState_MissingState_ReturnsDescriptiveError(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nvarSet{v: 1; value: 1}")
	_, err := StepState(evaluator.Context{}, 42, script, allExist)
	if err == nil || !strings.Contains(err.Error(), "42") {
		t.Errorf("error = %v, want one naming state 42", err)
	}
}

func TestStepState_BlockWithParseError_ErrorNamesTheBlock(t *testing.T) {
	script := mustParseScript(t, "[Statedef 9]\nfor i = 0, 3 {\n}")
	_, err := StepState(evaluator.Context{}, 9, script, allExist)
	if err == nil || !strings.Contains(err.Error(), "Statedef 9") {
		t.Errorf("error = %v, want one naming \"Statedef 9\"", err)
	}
}

func TestStepState_PlaySnd_ReturnsTheSoundAndLeavesNoneInTheContext(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nplaySnd{value: 5, 2}")
	result, err := StepState(evaluator.Context{}, 0, script, allExist)
	if err != nil {
		t.Fatalf("StepState returned unexpected error: %v", err)
	}
	if len(result.Sounds) != 1 || result.Sounds[0] != (evaluator.SoundEvent{Group: 5, Sample: 2, Channel: -1}) {
		t.Errorf("Sounds = %v, want one {5 2} on the default channel -1", result.Sounds)
	}
	if result.Context.Sounds != nil {
		t.Errorf("Context.Sounds = %v, want nil (the log is per call)", result.Context.Sounds)
	}
}

// Ikemen GO StateBlock.Run (src/bytecode.go:5136-5260, commit 54d4148a): in
// a hit pause a block without ignorehitpause does not run, and the
// statements of a block run only when that block has ctrlsIgnorehitpause.
func TestStepStateDuringHitPause_OnlyIgnoreHitPauseBlocksRun(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\n"+
		"varSet{v: 0; value: 1}\n"+ // root-level statement: skipped
		"ignoreHitPause {\n varSet{v: 1; value: 1}\n}\n"+ // runs
		"if 1 {\n varSet{v: 2; value: 1}\n}\n"+ // block without the flag: skipped
		"ignoreHitPause if 1 {\n varSet{v: 3; value: 1}\n}\n") // runs

	result, err := StepStateDuringHitPause(evaluator.Context{}, 0, script, allExist)
	if err != nil {
		t.Fatalf("StepStateDuringHitPause returned unexpected error: %v", err)
	}
	got := [4]int{result.Context.Vars[0], result.Context.Vars[1], result.Context.Vars[2], result.Context.Vars[3]}
	if want := [4]int{0, 1, 0, 1}; got != want {
		t.Errorf("var(0..3) = %v, want %v", got, want)
	}
}

func TestStepStateDuringHitPause_OuterBlockOfAFlaggedInnerBlock_RunsOnlyTheInnerOne(t *testing.T) {
	// The flag propagates up (compiler.go:7530), so the outer `if` is
	// entered and its condition evaluated; its own controllers are skipped.
	script := mustParseScript(t, "[Statedef 0]\n"+
		"if 1 {\n varSet{v: 0; value: 1}\n ignoreHitPause {\n  varSet{v: 1; value: 1}\n }\n}")
	result, err := StepStateDuringHitPause(evaluator.Context{}, 0, script, allExist)
	if err != nil {
		t.Fatalf("StepStateDuringHitPause returned unexpected error: %v", err)
	}
	if result.Context.Vars[0] != 0 || result.Context.Vars[1] != 1 {
		t.Errorf("var(0), var(1) = %d, %d, want 0, 1", result.Context.Vars[0], result.Context.Vars[1])
	}
}

func TestStepStateDuringHitPause_SkippedBlock_KeepsItsPersistentCounter(t *testing.T) {
	// StateBlock.Run returns before touching the counter (bytecode.go:5113-5120).
	script := mustParseScript(t, "[Statedef 0]\npersistent(3) if 1 {\n varSet{v: 0; value: 1}\n}")
	ctx := evaluator.Context{}
	ctx.Persistent[0] = 2

	result, err := StepStateDuringHitPause(ctx, 0, script, allExist)
	if err != nil {
		t.Fatalf("StepStateDuringHitPause returned unexpected error: %v", err)
	}
	if result.Context.Persistent[0] != 2 {
		t.Errorf("Persistent[0] = %d, want 2 (untouched)", result.Context.Persistent[0])
	}
}

func TestStepStateDuringHitPause_FunctionCallInAFlaggedBlock_RunsTheFunctionBody(t *testing.T) {
	// A Function's blocks all carry ignorehitpause (compiler.go:7852), but
	// the `call` itself is a statement like any other.
	script := mustParseScript(t,
		"[Function F()]\nvarSet{v: 0; value: 5}",
		"[Function G()]\nvarSet{v: 2; value: 9}",
		"[Statedef 0]\ncall G();\nignoreHitPause {\n call F();\n varSet{v: 1; value: 6}\n}",
	)
	result, err := StepStateDuringHitPause(evaluator.Context{}, 0, script, allExist)
	if err != nil {
		t.Fatalf("StepStateDuringHitPause returned unexpected error: %v", err)
	}
	got := [3]int{result.Context.Vars[0], result.Context.Vars[1], result.Context.Vars[2]}
	if want := [3]int{5, 6, 0}; got != want {
		t.Errorf("var(0..2) = %v, want %v (the unflagged root-level call G is skipped)", got, want)
	}
}

func TestStepStateDuringHitPause_FlaggedChangeState_Applies(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nignoreHitPause {\n changeState{value: 5}\n}", "[Statedef 5]")
	result, err := StepStateDuringHitPause(evaluator.Context{}, 0, script, allExist)
	if err != nil {
		t.Fatalf("StepStateDuringHitPause returned unexpected error: %v", err)
	}
	if result.Context.StateNo != 5 {
		t.Errorf("StateNo = %d, want 5", result.Context.StateNo)
	}
}

func TestCompile_StatedefHeader_KeepsTypesAndTheSourceTextOfEveryWrittenField(t *testing.T) {
	// Ikemen GO stateDef (src/compiler.go:7020-7130): type, movetype and
	// physics are letters; every other field is optional and evaluated on
	// entry (bytecode.go:5444-5512), so its text is kept and an omitted one
	// is absent.
	script := mustParseScript(t,
		"[Statedef 40; type: A; movetype: H; physics: N; anim: 41; poweradd: 5 * 2; velset: ifelse(1, 2, 3), -4; facep2: 1;]\n",
		"[Statedef 0]\n",
	)
	def, ok := script.Header(40)
	if !ok {
		t.Fatal("Header(40) not found")
	}
	if def.Number != 40 || def.Type != "A" || def.MoveType != "H" || def.Physics != "N" {
		t.Errorf("Header(40) number and types = %d %q %q %q, want 40 A H N", def.Number, def.Type, def.MoveType, def.Physics)
	}
	want := map[string]string{"anim": "41", "poweradd": "5 * 2", "velset.x": "ifelse(1, 2, 3)", "velset.y": "-4", "facep2": "1"}
	if !maps.Equal(def.HeaderExprs, want) {
		t.Errorf("HeaderExprs = %v, want %v (ctrl omitted, so absent)", def.HeaderExprs, want)
	}
	if _, ok := script.Header(99); ok {
		t.Error("Header(99) reported a state the script does not define")
	}
}

func TestCompile_SymbolicStatedefNumber_DoesNotDefineStateZero(t *testing.T) {
	// Number is 0 for a symbolic or relative number (zss.Block.NumberRaw);
	// such a block must not shadow the real state 0.
	script := mustParseScript(t,
		"[Statedef const(StateDizzy); type: A;]\nnull{}",
		"[Statedef 0; type: S;]\nnull{}",
	)
	def, ok := script.Header(0)
	if !ok || def.Type != "S" {
		t.Errorf("Header(0) = %+v, %v, want the real state 0 (type S)", def, ok)
	}
}

// blockExpressions calls f with every expression of stmts a .zss runs through
// the evaluator whole: block conditions, let values, call arguments and
// assigning statements. Controller parameters are left out: many hold
// several comma-separated values the controller splits itself.
func blockExpressions(stmts []stmt, f func(expr string)) {
	for _, s := range stmts {
		switch st := s.(type) {
		case *blockStmt:
			for b := st; b != nil; b = b.els {
				if b.hasCond {
					f(b.cond)
				}
				blockExpressions(b.body, f)
			}
		case letStmt:
			for _, e := range st.exprs {
				f(e)
			}
		case callStmt:
			for _, e := range st.args {
				f(e)
			}
		case exprStmt:
			f(st.expr)
		}
	}
}

// TestCompile_ReferenceScriptExpressions_AllParse parses every expression of
// Ikemen GO's common1.cns.zss and kfm_zss's scripts with the evaluator.
func TestCompile_ReferenceScriptExpressions_AllParse(t *testing.T) {
	for _, path := range []string{
		"../testdata/reference/common1.cns.zss",
		"../testdata/reference/chars/kfm_zss/kfm.zss",
		"../testdata/reference/chars/kfm_zss/AI.zss",
		"../testdata/reference/chars/kfm_zss/command.zss",
		"../testdata/reference/chars/kfm_zss/hits.zss",
	} {
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		script, err := zss.Parse(f)
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		count := 0
		for _, cb := range Compile(script).entries {
			blockExpressions(cb.stmts, func(expr string) {
				count++
				if _, err := evaluator.ParseShortCircuit(expr); err != nil {
					t.Errorf("%s, %s: %v", path, cb.desc, err)
				}
			})
		}
		if count == 0 {
			t.Errorf("%s: no expression found", path)
		}
	}
}
