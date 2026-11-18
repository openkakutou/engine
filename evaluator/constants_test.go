package evaluator

import (
	"math"
	"os"
	"strings"
	"testing"
)

// f32 is a value as Ikemen GO holds it: constants are float32 (char.go
// CharSize, CharVelocity, CharMovement), pushed on the float64 bytecode
// stack (bytecode.go PushF), so 2.4 reads as float32(2.4).
func f32(v float32) float64 { return float64(v) }

func loadConstants(t *testing.T, path string) *Constants {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c, err := ParseConstants(f)
	if err != nil {
		t.Fatalf("ParseConstants(%s): %v", path, err)
	}
	return c
}

func parseConstantsText(t *testing.T, text string) *Constants {
	t.Helper()
	c, err := ParseConstants(strings.NewReader(text))
	if err != nil {
		t.Fatalf("ParseConstants: %v", err)
	}
	return c
}

func constValue(t *testing.T, c *Constants, name string) Value {
	t.Helper()
	return mustEval(t, "Const("+name+")", Context{Consts: c})
}

// Every name Ikemen GO's compiler accepts in Const() (src/compiler.go:2246-2517,
// commit 54d4148a) has a value, so none falls through to the 0 of an unknown
// name.
var ikemenConstNames = []string{
	"data.life",
	"data.power",
	"data.dizzypoints",
	"data.guardpoints",
	"data.attack",
	"data.defence",
	"data.fall.defence_up",
	"data.fall.defence_mul",
	"data.liedown.time",
	"data.airjuggle",
	"data.sparkno",
	"data.guard.sparkno",
	"data.hitsound.channel",
	"data.guardsound.channel",
	"data.ko.echo",
	"data.volume",
	"data.intpersistindex",
	"data.floatpersistindex",
	"size.xscale",
	"size.yscale",
	"size.ground.back",
	"size.ground.front",
	"size.air.back",
	"size.air.front",
	"size.height",
	"size.attack.dist",
	"size.attack.dist.width.front",
	"size.attack.dist.width.back",
	"size.attack.dist.height.top",
	"size.attack.dist.height.bottom",
	"size.attack.dist.depth.top",
	"size.attack.dist.depth.bottom",
	"size.attack.depth.top",
	"size.attack.depth.bottom",
	"size.proj.attack.dist",
	"size.proj.attack.dist.width.front",
	"size.proj.attack.dist.width.back",
	"size.proj.attack.dist.height.top",
	"size.proj.attack.dist.height.bottom",
	"size.proj.attack.dist.depth.top",
	"size.proj.attack.dist.depth.bottom",
	"size.proj.doscale",
	"size.head.pos.x",
	"size.head.pos.y",
	"size.mid.pos.x",
	"size.mid.pos.y",
	"size.shadowoffset",
	"size.draw.offset.x",
	"size.draw.offset.y",
	"size.depth.top",
	"size.depth.bottom",
	"size.weight",
	"size.pushfactor",
	"velocity.air.gethit.airrecover.add.x",
	"velocity.air.gethit.airrecover.add.y",
	"velocity.air.gethit.airrecover.back",
	"velocity.air.gethit.airrecover.down",
	"velocity.air.gethit.airrecover.fwd",
	"velocity.air.gethit.airrecover.mul.x",
	"velocity.air.gethit.airrecover.mul.y",
	"velocity.air.gethit.airrecover.up",
	"velocity.air.gethit.groundrecover.x",
	"velocity.air.gethit.groundrecover.y",
	"velocity.air.gethit.ko.add.x",
	"velocity.air.gethit.ko.add.y",
	"velocity.air.gethit.ko.ymin",
	"velocity.airjump.back.x",
	"velocity.airjump.down.x",
	"velocity.airjump.down.y",
	"velocity.airjump.down.z",
	"velocity.airjump.fwd.x",
	"velocity.airjump.neu.x",
	"velocity.airjump.up.x",
	"velocity.airjump.up.y",
	"velocity.airjump.up.z",
	"velocity.airjump.y",
	"velocity.ground.gethit.ko.add.x",
	"velocity.ground.gethit.ko.add.y",
	"velocity.ground.gethit.ko.xmul",
	"velocity.ground.gethit.ko.ymin",
	"velocity.jump.back.x",
	"velocity.jump.down.x",
	"velocity.jump.down.y",
	"velocity.jump.down.z",
	"velocity.jump.fwd.x",
	"velocity.jump.neu.x",
	"velocity.jump.up.x",
	"velocity.jump.up.y",
	"velocity.jump.up.z",
	"velocity.jump.y",
	"velocity.run.back.x",
	"velocity.run.back.y",
	"velocity.run.down.x",
	"velocity.run.down.y",
	"velocity.run.down.z",
	"velocity.run.fwd.x",
	"velocity.run.fwd.y",
	"velocity.run.up.x",
	"velocity.run.up.y",
	"velocity.run.up.z",
	"velocity.runjump.back.x",
	"velocity.runjump.back.y",
	"velocity.runjump.down.x",
	"velocity.runjump.down.y",
	"velocity.runjump.down.z",
	"velocity.runjump.fwd.x",
	"velocity.runjump.up.x",
	"velocity.runjump.up.y",
	"velocity.runjump.up.z",
	"velocity.runjump.y",
	"velocity.walk.back.x",
	"velocity.walk.down.x",
	"velocity.walk.down.y",
	"velocity.walk.down.z",
	"velocity.walk.fwd.x",
	"velocity.walk.up.x",
	"velocity.walk.up.y",
	"velocity.walk.up.z",
	"movement.airjump.num",
	"movement.airjump.height",
	"movement.yaccel",
	"movement.stand.friction",
	"movement.crouch.friction",
	"movement.stand.friction.threshold",
	"movement.crouch.friction.threshold",
	"movement.air.gethit.groundlevel",
	"movement.air.gethit.groundrecover.ground.threshold",
	"movement.air.gethit.groundrecover.groundlevel",
	"movement.air.gethit.airrecover.threshold",
	"movement.air.gethit.airrecover.yaccel",
	"movement.air.gethit.trip.groundlevel",
	"movement.down.bounce.offset.x",
	"movement.down.bounce.offset.y",
	"movement.down.bounce.yaccel",
	"movement.down.bounce.groundlevel",
	"movement.down.gethit.offset.x",
	"movement.down.gethit.offset.y",
	"movement.down.friction.threshold",
}

func TestConstants_EveryIkemenConstName_HasAValueEvenWithoutAFile(t *testing.T) {
	for _, name := range ikemenConstNames {
		if _, ok := defaultConstants.builtIn[name]; !ok {
			t.Errorf("Const(%s) has no default: Ikemen GO accepts this name (compiler.go:2246-2517)", name)
		}
	}
}

// Values read off testdata/reference/chars/kfm/kfm.cns by hand ([Data],
// [Size], [Velocity], [Movement] sections), run through Ikemen GO's
// derivations (char.go:4411-4631) and float32 storage.
func TestConstants_KfmCns_ReadsTheCharactersOwnValues(t *testing.T) {
	c := loadConstants(t, "../testdata/reference/chars/kfm/kfm.cns")
	cases := []struct {
		name string
		want float64
		kind Kind
	}{
		{"data.life", 1000, KindInt},
		{"data.attack", 100, KindInt},
		{"data.fall.defence_up", 50, KindInt},
		// 1 / ((50+100)/100) in float32, bytecode.go OC_const_data_fall_defence_mul.
		{"data.fall.defence_mul", f32(float32(1.0) / float32(1.5)), KindFloat},
		{"data.liedown.time", 60, KindInt},
		{"data.sparkno", 2, KindInt},
		{"data.guard.sparkno", 40, KindInt},
		// volume = 0 -> 0/2 + 256 (char.go:4453).
		{"data.volume", 256, KindInt},
		{"data.intpersistindex", 60, KindInt},
		{"data.floatpersistindex", 40, KindInt},
		// absent from kfm.cns: Ikemen GO's defaults (char.go:329-350).
		{"data.power", 3000, KindInt},
		{"data.dizzypoints", 1000, KindInt},
		{"size.ground.back", 15, KindFloat},
		{"size.ground.front", 16, KindFloat},
		{"size.air.back", 12, KindFloat},
		{"size.height", 60, KindFloat},
		{"size.attack.dist", 160, KindFloat},
		{"size.attack.dist.width.front", 160, KindFloat},
		{"size.proj.attack.dist", 90, KindFloat},
		{"size.head.pos.x", -5, KindFloat},
		{"size.head.pos.y", -90, KindFloat},
		{"size.mid.pos.y", -60, KindFloat},
		{"size.weight", 100, KindInt},
		{"velocity.walk.fwd.x", f32(2.4), KindFloat},
		{"velocity.walk.back.x", f32(-2.2), KindFloat},
		{"velocity.run.fwd.x", f32(4.6), KindFloat},
		{"velocity.run.back.y", f32(-3.8), KindFloat},
		{"velocity.jump.neu.x", 0, KindFloat},
		{"velocity.jump.y", f32(-8.4), KindFloat},
		{"velocity.jump.back.x", f32(-2.55), KindFloat},
		{"velocity.jump.fwd.x", f32(2.5), KindFloat},
		{"velocity.runjump.back.x", f32(-2.55), KindFloat},
		{"velocity.runjump.y", f32(-8.1), KindFloat},
		{"velocity.runjump.fwd.x", 4, KindFloat},
		{"velocity.airjump.y", f32(-8.1), KindFloat},
		{"velocity.air.gethit.groundrecover.x", f32(-0.15), KindFloat},
		{"velocity.air.gethit.airrecover.mul.y", f32(0.2), KindFloat},
		{"velocity.air.gethit.airrecover.add.y", f32(-4.5), KindFloat},
		{"velocity.ground.gethit.ko.xmul", f32(0.66), KindFloat}, // default
		{"movement.airjump.num", 1, KindInt},
		{"movement.airjump.height", 35, KindFloat},
		{"movement.yaccel", f32(0.44), KindFloat},
		{"movement.stand.friction", f32(0.85), KindFloat},
		{"movement.crouch.friction", f32(0.82), KindFloat},
		{"movement.stand.friction.threshold", 2, KindFloat},
		{"movement.crouch.friction.threshold", f32(0.05), KindFloat},
		{"movement.air.gethit.groundlevel", 25, KindFloat},
		{"movement.air.gethit.groundrecover.ground.threshold", -20, KindFloat},
		{"movement.down.bounce.offset.y", 20, KindFloat},
		{"movement.down.friction.threshold", f32(0.05), KindFloat},
		{"movement.down.gethit.offset.y", 15, KindFloat}, // default
	}
	for _, tc := range cases {
		got := constValue(t, c, tc.name)
		if got.Number != tc.want || got.Kind != tc.kind {
			t.Errorf("Const(%s) = %v (kind %v), want %v (kind %v)", tc.name, got.Number, got.Kind, tc.want, tc.kind)
		}
	}
}

// kfm_zss's constants live in kfm.const, the same numbers in another file.
func TestConstants_KfmConstFile_ReadsTheSameValues(t *testing.T) {
	c := loadConstants(t, "../testdata/reference/chars/kfm_zss/kfm.const")
	if got := constValue(t, c, "movement.yaccel"); got.Number != f32(0.44) {
		t.Errorf("Const(movement.yaccel) = %v, want float32(0.44)", got.Number)
	}
	if got := constValue(t, c, "velocity.walk.fwd.x"); got.Number != f32(2.4) {
		t.Errorf("Const(velocity.walk.fwd.x) = %v, want float32(2.4)", got.Number)
	}
}

// With no file at all (a nil table) every name reads Ikemen GO's default.
func TestConstants_NilTable_ReadsIkemenDefaults(t *testing.T) {
	cases := []struct {
		name string
		want float64
	}{
		{"movement.yaccel", f32(0.44)},              // char.go:539
		{"movement.stand.friction", f32(0.85)},      // char.go:540-541
		{"movement.stand.friction.threshold", 2},    // char.go:541
		{"movement.crouch.friction.threshold", 0},   // char.go:543: 0, not MUGEN's .05
		{"movement.air.gethit.groundlevel", 25},     // char.go:544
		{"movement.down.gethit.offset.y", 15},       // char.go:554
		{"size.ground.front", 16},                   // char.go:394 standbox
		{"size.air.back", 12},                       // char.go:397 airbox
		{"size.height", 60},                         // char.go:394
		{"size.attack.dist.height.top", 1000},       // char.go:399
		{"size.proj.attack.dist.depth.top", 10},     // char.go:402
		{"size.weight", 100},                        // char.go:413
		{"size.pushfactor", 1},                      // char.go:414
		{"velocity.walk.fwd.x", 0},                  // never initialised
		{"velocity.air.gethit.ko.add.x", -2.5},      // char.go:483
		{"velocity.ground.gethit.ko.ymin", -6},      // char.go:488
		{"data.life", 1000},                         // char.go:330
		{"data.hitsound.channel", -1},               // char.go:343
		{"data.intpersistindex", math.MaxInt32},     // char.go:347
		{"default.attack.lifetopowermul", f32(0.7)}, // char.go:8246
		{"legacy.cornerpushdefaults", -1},           // char.go:8258
		{"input.pauseonhitpause", 1},                // char.go:8260
	}
	for _, tc := range cases {
		if got := mustEval(t, "Const("+tc.name+")", Context{}); got.Number != tc.want {
			t.Errorf("Const(%s) = %v, want %v", tc.name, got.Number, tc.want)
		}
	}
}

// Ikemen GO reads the first section of each kind and, in it, the first
// occurrence of a key (common.go:1071-1091 Parse, char.go:4403 flags).
func TestParseConstants_RepeatedSectionsAndKeys_FirstOneWins(t *testing.T) {
	c := parseConstantsText(t, "[Movement]\nyaccel = 0.5\nyaccel = 0.9\n[movement]\nyaccel = 0.7\nstand.friction = 0.1\n")
	if got := constValue(t, c, "movement.yaccel"); got.Number != f32(0.5) {
		t.Errorf("yaccel = %v, want 0.5 (first key, first section)", got.Number)
	}
	if got := constValue(t, c, "movement.stand.friction"); got.Number != f32(0.85) {
		t.Errorf("stand.friction = %v, want the default 0.85 (second section ignored)", got.Number)
	}
}

// char.go:4486-4495: runjump and airjump default to the jump values once
// the [Velocity] section is read.
func TestParseConstants_VelocityWithOnlyJump_RunAndAirJumpFollowIt(t *testing.T) {
	c := parseConstantsText(t, "[Velocity]\njump.neu = 1,-9\njump.back = -3\njump.fwd = 3.5\n")
	for name, want := range map[string]float64{
		"velocity.jump.neu.x":     1,
		"velocity.jump.y":         -9,
		"velocity.runjump.back.x": -3,
		"velocity.runjump.fwd.x":  3.5,
		"velocity.runjump.y":      -9,
		"velocity.runjump.back.y": -9,
		"velocity.airjump.neu.x":  1,
		"velocity.airjump.y":      -9,
		"velocity.airjump.back.x": -3,
		"velocity.airjump.fwd.x":  3.5,
	} {
		if got := constValue(t, c, name); got.Number != want {
			t.Errorf("Const(%s) = %v, want %v", name, got.Number, want)
		}
	}
}

// char.go:4411-4440: a sizebox overrides the legacy ground/air/height keys;
// the back and the top are stored negated, so Const returns them positive.
func TestParseConstants_SizeBoxAndLegacyKeys_SizeBoxOverrides(t *testing.T) {
	c := parseConstantsText(t, "[Size]\nground.back = 15\nground.front = 16\nheight = 60\nstand.sizebox = -20,-70,18,0\nair.sizebox = -9,-60,11,0\n")
	for name, want := range map[string]float64{
		"size.ground.back":  20,
		"size.height":       70,
		"size.ground.front": 18,
		"size.air.back":     9,
		"size.air.front":    11,
	} {
		if got := constValue(t, c, name); got.Number != want {
			t.Errorf("Const(%s) = %v, want %v", name, got.Number, want)
		}
	}
}

// char.go:4413: dizzypoints and guardpoints default to the character's life.
func TestParseConstants_DataLifeOnly_PointsDefaultToLife(t *testing.T) {
	c := parseConstantsText(t, "[Data]\nlife = 800\nguardpoints = 300\n")
	for name, want := range map[string]float64{"data.life": 800, "data.dizzypoints": 800, "data.guardpoints": 300} {
		if got := constValue(t, c, name); got.Number != want {
			t.Errorf("Const(%s) = %v, want %v", name, got.Number, want)
		}
	}
}

// Ikemen GO compiles an unknown name to a lookup in the [Constants] section
// (compiler.go:2518-2521) that reads int 0 when absent (bytecode.go:3358-3360,
// 1469-1475); it is not an error.
func TestConstants_CustomSection_ReadsTypedValuesAndUnknownReadsZero(t *testing.T) {
	c := parseConstantsText(t, "[Constants]\nMy.Count = 3\nmy.ratio = 0.25\nmy.label = \"x\"\n")
	if got := constValue(t, c, "my.count"); got.Number != 3 || got.Kind != KindInt {
		t.Errorf("my.count = %v kind %v, want int 3", got.Number, got.Kind)
	}
	if got := constValue(t, c, "My.Ratio"); got.Number != 0.25 || got.Kind != KindFloat {
		t.Errorf("My.Ratio = %v kind %v, want float 0.25 (names are case-insensitive)", got.Number, got.Kind)
	}
	if got := constValue(t, c, "no.such.constant"); got.Number != 0 || got.Kind != KindInt {
		t.Errorf("an unknown name = %v kind %v, want int 0", got.Number, got.Kind)
	}
}

// A [Constants] key does not shadow a built-in name: those compile to their
// own opcodes (compiler.go:2246-2517), never the map.
func TestParseConstants_CustomKeyNamedLikeABuiltIn_DoesNotShadowIt(t *testing.T) {
	c := parseConstantsText(t, "[Constants]\nmovement.yaccel = 9\ndefault.attack.lifetopowermul = 0.9\n")
	if got := constValue(t, c, "movement.yaccel"); got.Number != f32(0.44) {
		t.Errorf("movement.yaccel = %v, want the built-in 0.44", got.Number)
	}
	if got := constValue(t, c, "default.attack.lifetopowermul"); got.Number != f32(0.9) {
		t.Errorf("default.attack.lifetopowermul = %v, want the file's 0.9 (a custom default, char.go:8246)", got.Number)
	}
}

func TestParseConstants_FileWithBOMCommentsAndCRLF_ParsesLikeAnyOther(t *testing.T) {
	c := parseConstantsText(t, "\ufeff; header\r\n[Movement] ;trailing\r\nyaccel = .5 ;c\r\n")
	if got := constValue(t, c, "movement.yaccel"); got.Number != 0.5 {
		t.Errorf("yaccel = %v, want 0.5", got.Number)
	}
}

func TestParseConstants_NonNumericCustomValue_IsAnErrorNamingTheKey(t *testing.T) {
	_, err := ParseConstants(strings.NewReader("[Constants]\nbad.one = abc\n"))
	if err == nil || !strings.Contains(err.Error(), "bad.one") {
		t.Errorf("error = %v, want one naming bad.one", err)
	}
}

func TestEval_ConstWithBadArguments_IsAnError(t *testing.T) {
	for _, expr := range []string{"Const()", "Const(1)", "Const(life, power)", "Const(Life + 1)"} {
		e, err := Parse(expr)
		if err != nil {
			continue // rejected earlier still counts
		}
		if _, err := e.Eval(Context{}); err == nil {
			t.Errorf("Eval(%q) succeeded, want an error", expr)
		}
	}
}

// compiler.go:2530-2545 and char.go:5746 constp: localcoord[0] / coordinate
// * value; a 320-wide character gives value, value/2, value/4 and value/6.
func TestEval_ConstScalingFunctions_ScaleFromTheCharactersCoordinates(t *testing.T) {
	cases := []struct {
		expr string
		want float64
	}{
		{"Const240p(-2)", -2},
		{"Const240p(1)", 1},
		{"Const480p(10)", 5},
		{"Const720p(8)", 2},
		{"Const1080p(6)", 1},
	}
	for _, tc := range cases {
		got := mustEval(t, tc.expr, Context{})
		if got.Number != tc.want || got.Kind != KindFloat {
			t.Errorf("%s = %v kind %v, want float %v", tc.expr, got.Number, got.Kind, tc.want)
		}
	}
	if _, err := Evaluate("Const240p(1, 2)", Context{}); err == nil {
		t.Error("Const240p with two arguments returned no error")
	}
}

func TestTriggerNames_Const_ListsConstNotItsArgumentName(t *testing.T) {
	names, err := TriggerNames("Const(movement.yaccel) > 0")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != (TriggerName{Name: "const", Call: true}) {
		t.Errorf("TriggerNames = %v, want only const()", names)
	}
}

func TestEval_ConstWarmPath_AllocatesNothing(t *testing.T) {
	ctx := &Context{Consts: parseConstantsText(t, "[Movement]\nyaccel = .5\n")}
	if _, err := EvaluateMut("Const(movement.yaccel) + Const240p(1)", ctx); err != nil {
		t.Fatal(err)
	}
	allocs := testing.AllocsPerRun(100, func() { _, _ = EvaluateMut("Const(movement.yaccel) + Const240p(1)", ctx) })
	if allocs != 0 {
		t.Errorf("allocated %v times per call, want 0", allocs)
	}
}
