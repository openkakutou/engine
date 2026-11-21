package zssexec

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/v2/evaluator"
)

// stepN runs Step n times, feeding each result's Context into the next call,
// and returns the Context after every tick.
func stepN(t *testing.T, script CompiledScript, ctx evaluator.Context, n int) []evaluator.Context {
	t.Helper()
	var out []evaluator.Context
	for i := 0; i < n; i++ {
		r, err := Step(ctx, script)
		if err != nil {
			t.Fatalf("tick %d: Step returned unexpected error: %v", i+1, err)
		}
		ctx = r.Context
		out = append(out, ctx)
	}
	return out
}

func stepErr(t *testing.T, ctx evaluator.Context, blocks ...string) error {
	t.Helper()
	_, err := Step(ctx, mustParseScript(t, blocks...))
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	return err
}

func TestCompile_Common1Reference_EveryBlockParses(t *testing.T) {
	raw, err := os.ReadFile("../testdata/reference/common1.cns.zss")
	if err != nil {
		t.Fatalf("reading reference script: %v", err)
	}
	// Independent count: header lines, the way `grep -ci '^\[statedef\|^\[function'` sees them.
	headers := 0
	for _, line := range strings.Split(string(raw), "\n") {
		l := strings.ToLower(line)
		if strings.HasPrefix(l, "[statedef") || strings.HasPrefix(l, "[function") {
			headers++
		}
	}
	if headers != 62 {
		t.Fatalf("header lines = %d, want 62 (53 Statedef + 9 Function)", headers)
	}
	script, err := zss.Parse(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatalf("zss.Parse: %v", err)
	}
	prog := Compile(script)
	if len(script.Blocks) != headers {
		t.Fatalf("zss.Parse found %d blocks, want %d", len(script.Blocks), headers)
	}
	if got := prog.blocksParsed(); got != headers {
		t.Errorf("blocks compiled = %d, want %d", got, headers)
	}
	for _, e := range prog.Errors() {
		t.Errorf("block failed to parse: %v", e)
	}
}

// Mirrors Ikemen GO (commit 54d4148a) subBlock, src/compiler.go:7850, and the
// else handling of stateBlock/subBlock (7911-7925): `else if` is an else
// block that is itself an if block.
func TestStep_ElseIfChain_RunsTheMatchingBranchOnly(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\n"+
		"if var(0) = 1 {\n varSet{v: 1; value: 11;}\n} else if var(0) = 2 {\n varSet{v: 1; value: 22;}\n"+
		"} else {\n varSet{v: 1; value: 33;}\n}")
	for _, tc := range []struct{ in, want int }{{1, 11}, {2, 22}, {3, 33}} {
		ctx := evaluator.Context{}
		ctx.Vars[0] = tc.in
		r, err := Step(ctx, script)
		if err != nil {
			t.Fatal(err)
		}
		if r.Context.Vars[1] != tc.want {
			t.Errorf("var(0)=%d: var(1) = %d, want %d", tc.in, r.Context.Vars[1], tc.want)
		}
	}
}

// Mirrors Ikemen GO (commit 54d4148a) readSentence, src/compiler.go:7598-7660:
// a sentence is read across lines until a ';', '{' or '}' token.
func TestStep_ConditionAndControllerSpanningLines_Apply(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\n"+
		"if var(0) = 1 &&\n   var(1) >= ifElse(var(0) = 1,\n      2, 9) {\n"+
		"  varSet{\n    v: 2;\n    value: ifElse(var(0) = 1,\n         7, 8);\n  }\n}")
	ctx := evaluator.Context{}
	ctx.Vars[0], ctx.Vars[1] = 1, 2
	r, err := Step(ctx, script)
	if err != nil {
		t.Fatal(err)
	}
	if r.Context.Vars[2] != 7 {
		t.Errorf("var(2) = %d, want 7", r.Context.Vars[2])
	}
}

func TestStep_AssignInCondition_AssignsAndBranches(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nif sysVar(0) := (var(0) && 1) {\n varSet{v: 1; value: 5;}\n}")
	ctx := evaluator.Context{}
	ctx.Vars[0] = 1
	r, err := Step(ctx, script)
	if err != nil {
		t.Fatal(err)
	}
	if r.Context.SysVars[0] != 1 || r.Context.Vars[1] != 5 {
		t.Errorf("sysvar(0)=%d var(1)=%d, want 1 and 5", r.Context.SysVars[0], r.Context.Vars[1])
	}
	ctx.Vars[0] = 0
	ctx.SysVars[0] = 9
	r, err = Step(ctx, script)
	if err != nil {
		t.Fatal(err)
	}
	if r.Context.SysVars[0] != 0 || r.Context.Vars[1] != 0 {
		t.Errorf("sysvar(0)=%d var(1)=%d, want 0 (assigned, branch not taken) and 0", r.Context.SysVars[0], r.Context.Vars[1])
	}
}

func TestStep_AssignExpressionStatement_Assigns(t *testing.T) {
	r, err := Step(evaluator.Context{}, mustParseScript(t, "[Statedef 0]\nsysVar(1) := 7;"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Context.SysVars[1] != 7 {
		t.Errorf("sysvar(1) = %d, want 7", r.Context.SysVars[1])
	}
}

func TestStep_ExpressionStatementWithoutAssignment_IsAnError(t *testing.T) {
	err := stepErr(t, evaluator.Context{}, "[Statedef 0]\nvar(0) = 1;")
	for _, want := range []string{"Statedef 0", "unused value", "var(0) = 1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestStep_LetThenLocalInController_UsesTheLocal(t *testing.T) {
	r := stepBody(t, evaluator.Context{}, "let x = 21;", "varSet{v: 0; value: $x * 2}")
	if r.Context.Vars[0] != 42 {
		t.Errorf("var(0) = %d, want 42", r.Context.Vars[0])
	}
}

func TestStep_LetMultipleTargets_AssignsEach(t *testing.T) {
	r := stepBody(t, evaluator.Context{}, "let a, _, b = 2, 99, ifElse(1, 3, 4);", "varSet{v: 0; value: $a * 10 + $b}")
	if r.Context.Vars[0] != 23 {
		t.Errorf("var(0) = %d, want 23", r.Context.Vars[0])
	}
}

func TestStep_LetFromCall_ReceivesTheReturnValue(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\nlet a = call Triple(3);\nvarSet{v: 0; value: $a}",
		"[Function Triple(n) r]\nlet r = $n * 3;",
	)
	r, err := Step(evaluator.Context{}, script)
	if err != nil {
		t.Fatal(err)
	}
	if r.Context.Vars[0] != 9 {
		t.Errorf("var(0) = %d, want 9", r.Context.Vars[0])
	}
}

func TestStep_LetMultipleReturns_AssignsInOrder(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\nlet p, q = call Two(5);\nvarSet{v: 0; value: $p * 100 + $q}",
		"[Function Two(n) first, second]\nlet first = $n + 1;\nlet second = $n + 2;",
	)
	r, err := Step(evaluator.Context{}, script)
	if err != nil {
		t.Fatal(err)
	}
	if r.Context.Vars[0] != 607 {
		t.Errorf("var(0) = %d, want 607", r.Context.Vars[0])
	}
}

func TestStep_FunctionFrame_IsFreshAndZeroedOnEveryCall(t *testing.T) {
	// Stale holds a running let: a frame reused without being zeroed would
	// give 5 + 3 = 8 on the second call.
	script := mustParseScript(t,
		"[Statedef 0]\nlet a = call Stale(5);\nlet b = call Stale(3);\nvarSet{v: 0; value: $a}\nvarSet{v: 1; value: $b}",
		"[Function Stale(n) r]\nlet t = $t + $n;\nlet r = $t;",
	)
	r, err := Step(evaluator.Context{}, script)
	if err != nil {
		t.Fatal(err)
	}
	if r.Context.Vars[0] != 5 || r.Context.Vars[1] != 3 {
		t.Errorf("var(0)=%d var(1)=%d, want 5 and 3", r.Context.Vars[0], r.Context.Vars[1])
	}
}

func TestStep_CallerLocals_SurviveACallThatGrowsTheStack(t *testing.T) {
	// A deep chain of calls with big frames forces localStack to grow while
	// the caller's window is live.
	var lets strings.Builder
	for i := 0; i < 200; i++ {
		lets.WriteString("let v" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + " = 1;\n")
	}
	localStack = localStack[:0]
	script := mustParseScript(t,
		"[Statedef 0]\nlet keep = 77;\ncall Big();\nvarSet{v: 0; value: $keep}",
		"[Function Big()]\n"+lets.String()+"call Big2();",
		"[Function Big2()]\n"+lets.String(),
	)
	r, err := Step(evaluator.Context{}, script)
	if err != nil {
		t.Fatal(err)
	}
	if r.Context.Vars[0] != 77 {
		t.Errorf("var(0) = %d, want 77", r.Context.Vars[0])
	}
}

func TestStep_CallThatChangesState_DoesNotCopyReturnsBack(t *testing.T) {
	// The caller stops at the state change, so the copy-back is only
	// visible in the value stack: slot 0 ($a) must keep its old value, as
	// in Ikemen GO's BytecodeFunction.run (bytecode.go:5025).
	script := mustParseScript(t,
		"[Statedef 0]\nlet a = 1;\nlet a = call Go();",
		"[Function Go() r]\nlet r = 9;\nchangeState{value: 5}",
		"[Statedef 5]",
	)
	localStack = localStack[:0]
	r, err := Step(evaluator.Context{}, script)
	if err != nil {
		t.Fatal(err)
	}
	if r.Context.StateNo != 5 {
		t.Fatalf("StateNo = %d, want 5", r.Context.StateNo)
	}
	if got := localStack[0].Number; got != 1 {
		t.Errorf("caller's $a = %v after a state-changing call, want 1 (not 9)", got)
	}
}

func TestStep_StepEnd_LeavesNoLocalsInTheContext(t *testing.T) {
	r := stepBody(t, evaluator.Context{}, "let x = 1;", "varSet{v: 0; value: $x}")
	if r.Context.Locals.Names != nil || r.Context.Locals.Values != nil {
		t.Errorf("Locals = %+v, want empty", r.Context.Locals)
	}
}

func TestStep_PersistentZero_RunsOncePerStateEntry(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\n"+
		"persistent(0) {\n varSet{v: 0; value: var(0) + 1;}\n}\n"+
		"if var(1) = 1 {\n varSet{v: 1; value: 0;}\n changeState{value: 0;}\n}")
	ctx := evaluator.Context{}
	ticks := stepN(t, script, ctx, 2)
	if ticks[0].Vars[0] != 1 || ticks[1].Vars[0] != 1 {
		t.Fatalf("var(0) after ticks 1,2 = %d,%d, want 1,1", ticks[0].Vars[0], ticks[1].Vars[0])
	}
	// Re-enter the state (the script does it itself when var(1) = 1).
	ctx = ticks[1]
	ctx.Vars[1] = 1
	after := stepN(t, script, ctx, 3)
	got := []int{after[0].Vars[0], after[1].Vars[0], after[2].Vars[0]}
	want := []int{1, 2, 2} // tick 1 still suppressed, then re-entry re-arms it once
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("var(0) after re-entry tick %d = %d, want %d (all: %v)", i+1, got[i], want[i], got)
		}
	}
}

func TestStep_PersistentThree_RunsEveryThirdTick(t *testing.T) {
	// Ikemen GO StateBlock.Run: counter starts 0, runs, then is set to N and
	// decremented before each check, so ticks 1, 4, 7 run.
	script := mustParseScript(t, "[Statedef 0]\npersistent(3) {\n varSet{v: 0; value: var(0) + 1;}\n}")
	ticks := stepN(t, script, evaluator.Context{}, 7)
	want := []int{1, 1, 1, 2, 2, 2, 3}
	for i, c := range ticks {
		if c.Vars[0] != want[i] {
			t.Errorf("var(0) after tick %d = %d, want %d", i+1, c.Vars[0], want[i])
		}
	}
}

func TestStep_PersistentBeforeIf_GatesTheConditionToo(t *testing.T) {
	// persistent(0) if COND: the condition is not even evaluated once the
	// counter is armed (sysvar assignment would show it).
	script := mustParseScript(t, "[Statedef 0]\npersistent(0) if sysVar(0) := sysVar(0) + 1 {\n}")
	ticks := stepN(t, script, evaluator.Context{}, 3)
	// First run: condition true (1), body empty, counter armed: no more evaluations.
	if ticks[2].SysVars[0] != 1 {
		t.Errorf("sysvar(0) = %d, want 1 (condition evaluated once)", ticks[2].SysVars[0])
	}
}

func TestCompile_BadBodies_ReportOneErrorNamingTheBlockAndTheText(t *testing.T) {
	long := strings.Repeat("persistent(1) { }\n", evaluator.MaxPersistent+1)
	for _, tc := range []struct {
		name   string
		blocks []string
		wants  []string
	}{
		{"unknown local", []string{"[Statedef 0]\nvarSet{v: 0; value: $y}"}, []string{"Statedef 0", "$y"}},
		{"unknown local in function", []string{"[Function F(a)]\nlet b = $a + $zz;"}, []string{"Function f", "$zz"}},
		{"missing brace", []string{"[Statedef 3]\nif 1 {\n varSet{v: 0; value: 1}"}, []string{"Statedef 3", "}"}},
		{"for", []string{"[Statedef 0]\nfor i = 0, 3 {\n}"}, []string{"Statedef 0", "for", "not supported yet"}},
		{"while", []string{"[Statedef 0]\nwhile 1 {\n}"}, []string{"Statedef 0", "while", "not supported yet"}},
		{"switch", []string{"[Statedef 0]\nswitch 1 {\n}"}, []string{"switch", "not supported yet"}},
		{"persistent overflow", []string{"[Statedef 7]\n" + long}, []string{"Statedef 7", "persistent", strconv.Itoa(evaluator.MaxPersistent)}},
		{"persistent negative state", []string{"[Statedef -1]\npersistent(2) { }"}, []string{"Statedef -1", "negative"}},
		{"persistent in function", []string{"[Function F()]\npersistent(2) { }"}, []string{"Function f", "function"}},
		{"else without if", []string{"[Statedef 0]\nelse { }"}, []string{"Statedef 0", "else"}},
		{"duplicate ignoreHitPause", []string{"[Statedef 0]\nignoreHitPause ignoreHitPause { }"}, []string{"Statedef 0", "ignoreHitPause"}},
		{"duplicate persistent", []string{"[Statedef 0]\npersistent(1) persistent(2) { }"}, []string{"Statedef 0", "persistent"}},
		{"let name count", []string{"[Statedef 0]\nlet a, b = 1;"}, []string{"Statedef 0", "let a, b = 1"}},
		{"unterminated statement", []string{"[Statedef 0]\nsysVar(0) := 1"}, []string{"Statedef 0", "sysVar(0) := 1"}},
		{"unterminated string", []string{"[Statedef 0]\nvarSet{v: 0; value: \"abc}"}, []string{"Statedef 0", "string"}},
		{"duplicate controller key", []string{"[Statedef 0]\nvarSet{v: 0; v: 1}"}, []string{"Statedef 0", "duplicate"}},
		{"duplicate parameter", []string{"[Function F(a, a)]"}, []string{"Function f", "duplicate", "a"}},
		{"unused expression", []string{"[Statedef 0]\nvar(0) = 1;"}, []string{"Statedef 0", "unused value", "var(0) = 1"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errs := mustParseScript(t, tc.blocks...).Errors()
			if len(errs) != 1 {
				t.Fatalf("Errors() = %v, want exactly 1", errs)
			}
			for _, w := range tc.wants {
				if !strings.Contains(strings.ToLower(errs[0].Error()), strings.ToLower(w)) {
					t.Errorf("error %q does not contain %q", errs[0], w)
				}
			}
		})
	}
}

func TestStep_BlockWithParseError_ReturnsItsError(t *testing.T) {
	err := stepErr(t, evaluator.Context{}, "[Statedef 0]\nvarSet{v: 0; value: $nope}")
	if !strings.Contains(err.Error(), "$nope") {
		t.Errorf("error %q does not name $nope", err)
	}
}

func TestParse_IgnoreHitPause_FollowsIkemenInheritance(t *testing.T) {
	flags := func(t *testing.T, header, body string) []bool {
		t.Helper()
		prog := mustParseScript(t, header+"\n"+body)
		if errs := prog.Errors(); len(errs) > 0 {
			t.Fatal(errs)
		}
		var out []bool
		var walk func(stmts []stmt)
		walk = func(stmts []stmt) {
			for _, s := range stmts {
				if b, ok := s.(*blockStmt); ok {
					for ; b != nil; b = b.els {
						out = append(out, b.ignoreHitPause)
						walk(b.body)
					}
				}
			}
		}
		for _, b := range prog.entries {
			walk(b.stmts)
		}
		return out
	}
	eq := func(got, want []bool) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}
	cases := []struct {
		name         string
		header, body string
		want         []bool
	}{
		{"plain if", "[Statedef 0]", "if 1 {\n if 1 {\n }\n}", []bool{false, false}},
		{"inherited by nested blocks", "[Statedef 0]", "ignoreHitPause if 1 {\n if 1 {\n }\n}", []bool{true, true}},
		{"propagates up to the parent", "[Statedef 0]", "if 1 {\n ignoreHitPause {\n }\n}", []bool{true, true}},
		{"else branch propagates to the if", "[Statedef 0]", "if 1 {\n} else ignoreHitPause {\n}", []bool{true, true}},
		{"every function block", "[Function F()]", "if 1 {\n}", []bool{true}},
	}
	for _, tc := range cases {
		if got := flags(t, tc.header, tc.body); !eq(got, tc.want) {
			t.Errorf("%s: ignoreHitPause flags = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestStep_WarmBlockWithLetCallAndPersistent_DoesNotAllocate(t *testing.T) {
	script := mustParseScript(t,
		"[Statedef 0]\nlet x = 2;\npersistent(2) {\n let y = call Twice($x);\n sysVar(0) := $y;\n}\nvarSet{v: 0; value: $x}",
		"[Function Twice(n) r]\nlet r = $n * 2;",
	)
	ctx := evaluator.Context{}
	var err error
	for i := 0; i < 3; i++ { // warm the parse caches and the value stack
		var r Result
		if r, err = Step(ctx, script); err != nil {
			t.Fatal(err)
		}
		ctx = r.Context
	}
	allocs := testing.AllocsPerRun(100, func() {
		r, _ := Step(ctx, script)
		ctx = r.Context
	})
	if allocs != 0 {
		t.Errorf("AllocsPerRun = %v, want 0", allocs)
	}
	if ctx.SysVars[0] != 4 {
		t.Errorf("sysvar(0) = %d, want 4 (the block must really have run)", ctx.SysVars[0])
	}
}

func TestStep_UnderscoreParameter_TakesASlotSoLaterArgumentsDoNotShift(t *testing.T) {
	// Ikemen GO gives `_` its own slot (compiler.go:8509-8519).
	script := mustParseScript(t,
		"[Statedef 0]\nlet r = call f(1, 2);\nvarSet{v: 0; value: $r}",
		"[Function f(_, x) y]\nlet y = $x;",
	)
	localStack = nil
	r, err := Step(evaluator.Context{}, script)
	if err != nil {
		t.Fatal(err)
	}
	if r.Context.Vars[0] != 2 {
		t.Errorf("var(0) = %d, want 2", r.Context.Vars[0])
	}
}

func TestStep_CallOfFunctionWithOnlyUnderscoreParameter_DoesNotPanic(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\ncall f(5);", "[Function f(_)]\n")
	localStack = nil
	if _, err := Step(evaluator.Context{}, script); err != nil {
		t.Fatal(err)
	}
}

func TestParse_UnderscoreParameter_IsNotReadable(t *testing.T) {
	errs := mustParseScript(t, "[Function f(_)]\nlet a = $_;").Errors()
	if len(errs) != 1 {
		t.Fatalf("Errors() = %v, want 1", errs)
	}
}

// Ikemen GO evaluates .zss controller parameters with short-circuit
// (compiler.go:8410, expBoolAnd OC_jz 6042-6072): at Time 0 the right side of
// && is not evaluated, so Random is not drawn.
func TestStep_ZSSControllerParameters_ShortCircuit(t *testing.T) {
	ctx := evaluator.Context{RandSeed: 1}
	r := stepBody(t, ctx, "velSet{x: ifElse(time > 0 && random < 500, 1, 2)}")
	if r.Context.RandSeed != 1 {
		t.Errorf("RandSeed = %d, want 1 (right side of && skipped)", r.Context.RandSeed)
	}
	if r.Context.Velocity.X != 2 {
		t.Errorf("Velocity.X = %v, want 2", r.Context.Velocity.X)
	}
}

func TestStep_AfterStep_ContextIsNotLeftInZSSMode(t *testing.T) {
	r := stepBody(t, evaluator.Context{}, "velSet{x: 1}")
	if r.Context.ZSS {
		t.Error("Result.Context.ZSS = true: a later .cns evaluation would short-circuit")
	}
}

// Ikemen GO logs a call to an undefined function or with the wrong number of
// arguments or returns and does nothing (bytecode.go:5050-5069): the rest of
// the state keeps running.
func TestStep_WrongCalls_AreSkippedReportedAndTheStateKeepsRunning(t *testing.T) {
	for _, tc := range []struct{ name, call, fn string }{
		{"undefined", "call Nope();", "[Function Other()]\n"},
		{"too many arguments", "call F(1, 2);", "[Function F(a)]\n"},
		{"too few arguments", "call F();", "[Function F(a)]\n"},
		{"wrong return count", "let a, b = call F(1);", "[Function F(a) r]\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			script := mustParseScript(t, "[Statedef 0]\n"+tc.call+"\nchangeState{value: 5}", tc.fn, "[Statedef 5]")
			r, err := Step(evaluator.Context{}, script)
			if err != nil {
				t.Fatalf("Step returned %v, want the call skipped", err)
			}
			if r.Context.StateNo != 5 {
				t.Errorf("StateNo = %d, want 5 (ChangeState after the bad call must run)", r.Context.StateNo)
			}
			if len(r.SkippedCalls) != 1 || !strings.Contains(strings.ToLower(r.SkippedCalls[0]), "f") {
				t.Errorf("SkippedCalls = %v, want one entry naming the call", r.SkippedCalls)
			}
		})
	}
}

// kfm_zss's CSnd/SysSnd functions (kfm.zss:24,29): the prefix is spaced from
// a local, and the channel is a local too.
func TestStep_PlaySndWithSpacedPrefixAndLocals_RecordsTheEvent(t *testing.T) {
	r := stepBody(t, evaluator.Context{}, "let b, s, c = 5, 2, 1;", "playSnd{value: F $b, $s; channel: $c}\nplaySnd{value: S $b, $s}")
	want := []evaluator.SoundEvent{{Group: 5, Sample: 2, Common: true, Channel: 1}, {Group: 5, Sample: 2, Channel: -1}}
	if len(r.Sounds) != 2 || r.Sounds[0] != want[0] || r.Sounds[1] != want[1] {
		t.Errorf("Sounds = %v, want %v", r.Sounds, want)
	}
}

// compiler.go letAssign (8179) reads each value with the expression parser,
// which takes a redirection's comma with it: kfm_zss AI.zss state -1 has
// `let nojug = p2,anim = 5120 || ...`.
func TestStep_LetValueWithARedirectionCommaIsOneExpression(t *testing.T) {
	ctx := evaluator.Context{P2: evaluator.Opponent{Valid: true, Anim: 5120}}
	r := stepBody(t, ctx, "let nojug = p2,anim = 5120 || p2,anim = 5121;", "varSet{v: 0; value: $nojug}")
	if r.Context.Vars[0] != 1 {
		t.Errorf("var(0) = %d, want 1", r.Context.Vars[0])
	}
}

func TestStep_LetSeveralValuesWithRedirectionsSplitBetweenThem(t *testing.T) {
	ctx := evaluator.Context{P2: evaluator.Opponent{Valid: true, Anim: 7}}
	r := stepBody(t, ctx, "let a, b = p2,anim, enemy(0),anim;", "varSet{v: 0; value: $a * 1000 + $b}")
	if r.Context.Vars[0] != 7007 {
		t.Errorf("var(0) = %d, want 7007", r.Context.Vars[0])
	}
}

// compiler.go readKeyValue (7662-7693) stores a parameter's sentence even when
// it is empty; kfm_zss stops its counters with `reversalDef{reversal.attr: }`
// (kfm.zss:1962), an attr of 0 that catches nothing (attrMask, compiler.go:735).
func TestCompile_AnEmptyParameterValueIsKept(t *testing.T) {
	script := mustParseScript(t, "[Statedef 0]\nreversalDef{reversal.attr: }\nvarSet{v: 0; value: 3}")
	for _, cb := range script.entries {
		if cb.err != nil {
			t.Fatalf("compile error: %v", cb.err)
		}
	}
	var got map[string]string
	for _, st := range script.statedefs[0].stmts {
		if c, ok := st.(ctrlStmt); ok && strings.EqualFold(c.ctrl.Type, "reversalDef") {
			got = c.ctrl.Parameters
		}
	}
	if v, ok := got["reversal.attr"]; !ok || v != "" {
		t.Fatalf("parameters = %v, want reversal.attr kept empty", got)
	}
}
