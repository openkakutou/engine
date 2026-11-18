package evaluator

import (
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Constants is a character's constants table, the values Const(name) reads:
// the [Data], [Size], [Velocity] and [Movement] sections of the .def's "cns"
// file (kfm.cns, kfm.const) over Ikemen GO's defaults, plus the file's own
// [Constants] section. It is built once per character (ParseConstants) and
// never written afterwards, so a Context holds only a pointer to it and
// copying a Context stays cheap; physics reads the same table (backlog item
// 043). A nil *Constants reads as the defaults.
//
// Reference: Ikemen GO src/char.go (commit 54d4148a), initConstants
// :8120-8201 for the defaults, the "cns" loading :4372-4631 for the file.
type Constants struct {
	// builtIn holds the names Ikemen GO's compiler gives their own opcode
	// (compiler.go:2246-2517), all lower-case.
	builtIn map[string]Value
	// custom holds the [Constants] section, which Ikemen GO reads into the
	// map that Const() falls back to for any other name (compiler.go:2518,
	// char.go:4663-4668), seeded with initConstants' custom defaults.
	custom map[string]Value
}

// defaultConstants is what a nil *Constants reads.
var defaultConstants = newDefaultConstants()

// constantKey is one key of a constants-file section and the Const() names it
// fills, one name per comma-separated value, in order. ints marks the
// names Ikemen GO pushes as ints (bytecode.go PushI); every other name is a
// float32.
type constantKey struct {
	key   string
	names []string
	ints  bool
}

// Not every Const() name is its key plus a suffix (velocity.jump.neu's second
// value is velocity.jump.y), so the names are spelled out.
var dataKeys = []constantKey{
	{"life", []string{"data.life"}, true},
	{"power", []string{"data.power"}, true},
	{"dizzypoints", []string{"data.dizzypoints"}, true},
	{"guardpoints", []string{"data.guardpoints"}, true},
	{"attack", []string{"data.attack"}, true},
	{"defence", []string{"data.defence"}, true},
	{"fall.defence_up", []string{"data.fall.defence_up"}, true},
	{"liedown.time", []string{"data.liedown.time"}, true},
	{"airjuggle", []string{"data.airjuggle"}, true},
	{"sparkno", []string{"data.sparkno"}, true},
	{"guard.sparkno", []string{"data.guard.sparkno"}, true},
	{"hitsound.channel", []string{"data.hitsound.channel"}, true},
	{"guardsound.channel", []string{"data.guardsound.channel"}, true},
	{"ko.echo", []string{"data.ko.echo"}, true},
	{"intpersistindex", []string{"data.intpersistindex"}, true},
	{"floatpersistindex", []string{"data.floatpersistindex"}, true},
}

var sizeKeys = []constantKey{
	{"xscale", []string{"size.xscale"}, false},
	{"yscale", []string{"size.yscale"}, false},
	{"ground.front", []string{"size.ground.front"}, false},
	{"ground.back", []string{"size.ground.back"}, false},
	{"air.front", []string{"size.air.front"}, false},
	{"air.back", []string{"size.air.back"}, false},
	{"height", []string{"size.height"}, false},
	{"attack.dist", []string{"size.attack.dist.width.front"}, false},
	{"attack.dist.width", []string{"size.attack.dist.width.front", "size.attack.dist.width.back"}, false},
	{"attack.dist.height", []string{"size.attack.dist.height.top", "size.attack.dist.height.bottom"}, false},
	{"attack.dist.depth", []string{"size.attack.dist.depth.top", "size.attack.dist.depth.bottom"}, false},
	{"proj.attack.dist", []string{"size.proj.attack.dist.width.front"}, false},
	{"proj.attack.dist.width", []string{"size.proj.attack.dist.width.front", "size.proj.attack.dist.width.back"}, false},
	{"proj.attack.dist.height", []string{"size.proj.attack.dist.height.top", "size.proj.attack.dist.height.bottom"}, false},
	{"proj.attack.dist.depth", []string{"size.proj.attack.dist.depth.top", "size.proj.attack.dist.depth.bottom"}, false},
	{"proj.doscale", []string{"size.proj.doscale"}, true},
	{"head.pos", []string{"size.head.pos.x", "size.head.pos.y"}, false},
	{"mid.pos", []string{"size.mid.pos.x", "size.mid.pos.y"}, false},
	{"shadowoffset", []string{"size.shadowoffset"}, false},
	{"draw.offset", []string{"size.draw.offset.x", "size.draw.offset.y"}, false},
	{"depth", []string{"size.depth.top", "size.depth.bottom"}, false},
	{"attack.depth", []string{"size.attack.depth.top", "size.attack.depth.bottom"}, false},
	{"weight", []string{"size.weight"}, true},
	{"pushfactor", []string{"size.pushfactor"}, false},
}

// velocityJumpKeys come first: runjump and airjump default to the jump values
// before their own keys are read (char.go:4486-4495).
var velocityJumpKeys = []constantKey{
	{"jump.neu", []string{"velocity.jump.neu.x", "velocity.jump.y"}, false},
	{"jump.back", []string{"velocity.jump.back.x"}, false},
	{"jump.fwd", []string{"velocity.jump.fwd.x"}, false},
}

var velocityKeys = []constantKey{
	{"walk.fwd", []string{"velocity.walk.fwd.x"}, false},
	{"walk.back", []string{"velocity.walk.back.x"}, false},
	{"run.fwd", []string{"velocity.run.fwd.x", "velocity.run.fwd.y"}, false},
	{"run.back", []string{"velocity.run.back.x", "velocity.run.back.y"}, false},
	{"runjump.back", []string{"velocity.runjump.back.x", "velocity.runjump.back.y"}, false},
	{"runjump.fwd", []string{"velocity.runjump.fwd.x", "velocity.runjump.y"}, false},
	{"airjump.neu", []string{"velocity.airjump.neu.x", "velocity.airjump.y"}, false},
	{"airjump.back", []string{"velocity.airjump.back.x"}, false},
	{"airjump.fwd", []string{"velocity.airjump.fwd.x"}, false},
	{"air.gethit.groundrecover", []string{"velocity.air.gethit.groundrecover.x", "velocity.air.gethit.groundrecover.y"}, false},
	{"air.gethit.airrecover.mul", []string{"velocity.air.gethit.airrecover.mul.x", "velocity.air.gethit.airrecover.mul.y"}, false},
	{"air.gethit.airrecover.add", []string{"velocity.air.gethit.airrecover.add.x", "velocity.air.gethit.airrecover.add.y"}, false},
	{"air.gethit.airrecover.back", []string{"velocity.air.gethit.airrecover.back"}, false},
	{"air.gethit.airrecover.fwd", []string{"velocity.air.gethit.airrecover.fwd"}, false},
	{"air.gethit.airrecover.up", []string{"velocity.air.gethit.airrecover.up"}, false},
	{"air.gethit.airrecover.down", []string{"velocity.air.gethit.airrecover.down"}, false},
	{"air.gethit.ko.add", []string{"velocity.air.gethit.ko.add.x", "velocity.air.gethit.ko.add.y"}, false},
	{"air.gethit.ko.ymin", []string{"velocity.air.gethit.ko.ymin"}, false},
	{"ground.gethit.ko.xmul", []string{"velocity.ground.gethit.ko.xmul"}, false},
	{"ground.gethit.ko.add", []string{"velocity.ground.gethit.ko.add.x", "velocity.ground.gethit.ko.add.y"}, false},
	{"ground.gethit.ko.ymin", []string{"velocity.ground.gethit.ko.ymin"}, false},
	// Undocumented in MUGEN; Ikemen GO reads three axes (char.go:4549-4558).
	{"walk.up", []string{"velocity.walk.up.x", "velocity.walk.up.y", "velocity.walk.up.z"}, false},
	{"walk.down", []string{"velocity.walk.down.x", "velocity.walk.down.y", "velocity.walk.down.z"}, false},
	{"run.up", []string{"velocity.run.up.x", "velocity.run.up.y", "velocity.run.up.z"}, false},
	{"run.down", []string{"velocity.run.down.x", "velocity.run.down.y", "velocity.run.down.z"}, false},
	{"jump.up", []string{"velocity.jump.up.x", "velocity.jump.up.y", "velocity.jump.up.z"}, false},
	{"jump.down", []string{"velocity.jump.down.x", "velocity.jump.down.y", "velocity.jump.down.z"}, false},
	{"runjump.up", []string{"velocity.runjump.up.x", "velocity.runjump.up.y", "velocity.runjump.up.z"}, false},
	{"runjump.down", []string{"velocity.runjump.down.x", "velocity.runjump.down.y", "velocity.runjump.down.z"}, false},
	{"airjump.up", []string{"velocity.airjump.up.x", "velocity.airjump.up.y", "velocity.airjump.up.z"}, false},
	{"airjump.down", []string{"velocity.airjump.down.x", "velocity.airjump.down.y", "velocity.airjump.down.z"}, false},
}

var movementKeys = []constantKey{
	{"airjump.num", []string{"movement.airjump.num"}, true},
	{"airjump.height", []string{"movement.airjump.height"}, false},
	{"yaccel", []string{"movement.yaccel"}, false},
	{"stand.friction", []string{"movement.stand.friction"}, false},
	{"stand.friction.threshold", []string{"movement.stand.friction.threshold"}, false},
	{"crouch.friction", []string{"movement.crouch.friction"}, false},
	{"crouch.friction.threshold", []string{"movement.crouch.friction.threshold"}, false},
	{"air.gethit.groundlevel", []string{"movement.air.gethit.groundlevel"}, false},
	{"air.gethit.groundrecover.ground.threshold", []string{"movement.air.gethit.groundrecover.ground.threshold"}, false},
	{"air.gethit.groundrecover.groundlevel", []string{"movement.air.gethit.groundrecover.groundlevel"}, false},
	{"air.gethit.airrecover.threshold", []string{"movement.air.gethit.airrecover.threshold"}, false},
	{"air.gethit.airrecover.yaccel", []string{"movement.air.gethit.airrecover.yaccel"}, false},
	{"air.gethit.trip.groundlevel", []string{"movement.air.gethit.trip.groundlevel"}, false},
	{"down.bounce.offset", []string{"movement.down.bounce.offset.x", "movement.down.bounce.offset.y"}, false},
	{"down.bounce.yaccel", []string{"movement.down.bounce.yaccel"}, false},
	{"down.bounce.groundlevel", []string{"movement.down.bounce.groundlevel"}, false},
	{"down.friction.threshold", []string{"movement.down.friction.threshold"}, false},
	{"down.gethit.offset", []string{"movement.down.gethit.offset.x", "movement.down.gethit.offset.y"}, false},
}

// constantDefaults are the non-zero defaults of initConstants (char.go:329-
// 350 data, :391-418 size, :477-490 velocity, :538-556 movement). A name
// listed in a key table and absent here defaults to 0.
var constantDefaults = map[string]float64{
	"data.life": 1000, "data.power": 3000, "data.dizzypoints": 1000, "data.guardpoints": 1000,
	"data.attack": 100, "data.defence": 100, "data.fall.defence_up": 50,
	"data.liedown.time": 60, "data.airjuggle": 15, "data.sparkno": 2, "data.guard.sparkno": 40,
	"data.hitsound.channel": -1, "data.guardsound.channel": -1, "data.volume": 256,
	"data.intpersistindex": 2147483647, "data.floatpersistindex": 2147483647,

	"size.xscale": 1, "size.yscale": 1,
	"size.ground.back": 16, "size.ground.front": 16, "size.air.back": 12, "size.air.front": 12, "size.height": 60,
	"size.attack.dist.width.front": 160, "size.attack.dist.height.top": 1000, "size.attack.dist.height.bottom": 1000,
	"size.attack.dist.depth.top": 4, "size.attack.dist.depth.bottom": 4,
	"size.proj.attack.dist.width.front": 90, "size.proj.attack.dist.height.top": 1000, "size.proj.attack.dist.height.bottom": 1000,
	"size.proj.attack.dist.depth.top": 10, "size.proj.attack.dist.depth.bottom": 10,
	"size.head.pos.x": -5, "size.head.pos.y": -90, "size.mid.pos.x": -5, "size.mid.pos.y": -60,
	"size.depth.top": 4, "size.depth.bottom": 4, "size.attack.depth.top": 4, "size.attack.depth.bottom": 4,
	"size.weight": 100, "size.pushfactor": 1,

	"velocity.air.gethit.groundrecover.x": -0.15, "velocity.air.gethit.groundrecover.y": -3.5,
	"velocity.air.gethit.airrecover.mul.x": 0.5, "velocity.air.gethit.airrecover.mul.y": 0.2,
	"velocity.air.gethit.airrecover.add.y": -4.5,
	"velocity.air.gethit.airrecover.back":  -1, "velocity.air.gethit.airrecover.up": -2, "velocity.air.gethit.airrecover.down": 1.5,
	"velocity.air.gethit.ko.add.x": -2.5, "velocity.air.gethit.ko.add.y": -2, "velocity.air.gethit.ko.ymin": -3,
	"velocity.ground.gethit.ko.xmul": 0.66, "velocity.ground.gethit.ko.add.x": -2.5,
	"velocity.ground.gethit.ko.add.y": -2, "velocity.ground.gethit.ko.ymin": -6,

	"movement.yaccel": 0.44, "movement.stand.friction": 0.85, "movement.stand.friction.threshold": 2,
	"movement.crouch.friction": 0.82, "movement.air.gethit.groundlevel": 25,
	"movement.air.gethit.groundrecover.ground.threshold": -20, "movement.air.gethit.groundrecover.groundlevel": 10,
	"movement.air.gethit.airrecover.threshold": -1, "movement.air.gethit.airrecover.yaccel": 0.35,
	"movement.air.gethit.trip.groundlevel": 15, "movement.down.bounce.offset.y": 20,
	"movement.down.bounce.yaccel": 0.4, "movement.down.bounce.groundlevel": 12,
	"movement.down.gethit.offset.y": 15, "movement.down.friction.threshold": 0.05,
}

// customDefaults are initConstants' defaults for the custom constants the
// engine itself reads (char.go:8246-8261).
var customDefaults = map[string]Value{
	"default.attack.lifetopowermul": floatValue(float64(float32(0.7))),
	"super.attack.lifetopowermul":   floatValue(0),
	"default.gethit.lifetopowermul": floatValue(float64(float32(0.6))),
	"super.gethit.lifetopowermul":   floatValue(float64(float32(0.6))),
	"super.targetdefencemul":        floatValue(1.5),
	"default.lifetoguardpointsmul":  floatValue(1.5),
	"super.lifetoguardpointsmul":    floatValue(float64(float32(-0.33))),
	"default.lifetodizzypointsmul":  floatValue(float64(float32(1.8))),
	"super.lifetodizzypointsmul":    floatValue(0),
	"default.lifetoredlifemul":      floatValue(0.75),
	"super.lifetoredlifemul":        floatValue(0.75),
	"legacy.gamedistancespec":       intValue(0),
	"legacy.fallyvelyaccel":         floatValue(0),
	"legacy.cornerpushdefaults":     floatValue(-1),
	"input.pauseonhitpause":         intValue(1),
	"input.fbflipenemydistance":     floatValue(-1),
}

func newDefaultConstants() *Constants {
	c := &Constants{builtIn: map[string]Value{}, custom: make(map[string]Value, len(customDefaults))}
	for _, keys := range [][]constantKey{dataKeys, sizeKeys, velocityJumpKeys, velocityKeys, movementKeys} {
		for _, k := range keys {
			for _, name := range k.names {
				c.set(name, constantDefaults[name], k.ints)
			}
		}
	}
	c.set("data.volume", constantDefaults["data.volume"], true)
	c.set("data.fall.defence_mul", 0, false)
	c.setAttackDistAliases()
	c.setDefenceMul()
	for name, v := range customDefaults {
		c.custom[name] = v
	}
	return c
}

func (c *Constants) set(name string, v float64, isInt bool) {
	if isInt {
		c.builtIn[name] = intValue(math.Trunc(v))
		return
	}
	c.builtIn[name] = floatValue(float64(float32(v)))
}

// setAttackDistAliases fills Const(size.attack.dist), which Ikemen GO accepts
// for size.attack.dist.width.front (compiler.go:2297), and the projectile one.
func (c *Constants) setAttackDistAliases() {
	c.builtIn["size.attack.dist"] = c.builtIn["size.attack.dist.width.front"]
	c.builtIn["size.proj.attack.dist"] = c.builtIn["size.proj.attack.dist.width.front"]
}

// setDefenceMul fills Const(data.fall.defence_mul), 1 / defence_mul in
// float32 (bytecode.go OC_const_data_fall_defence_mul), where defence_mul is
// (fall.defence_up + 100) / 100 (char.go:4439).
func (c *Constants) setDefenceMul() {
	mul := (float32(c.builtIn["data.fall.defence_up"].Number) + 100) / 100
	c.builtIn["data.fall.defence_mul"] = floatValue(float64(1.0 / mul))
}

// Lookup returns the value of Const(name), name lower-case. A name Ikemen GO
// does not know reads int 0 (bytecode.go:1469-1475).
func (c *Constants) Lookup(name string) Value {
	if c == nil {
		c = defaultConstants
	}
	if v, ok := c.builtIn[name]; ok {
		return v
	}
	if v, ok := c.custom[name]; ok {
		return v
	}
	return intValue(0)
}

// ParseConstants reads a character's constants file (the .def's "cns"
// entry) over Ikemen GO's defaults. Like Ikemen GO it reads only the first
// [Data], [Size], [Velocity], [Movement] and [Constants] section, and only
// the first occurrence of a key in a section (common.go:1071-1091); other
// sections (states, quotes) are skipped. A value that does not parse is read
// as far as it is numeric, as Ikemen GO's Atof/Atoi do. A [Constants] value
// that is neither a number nor a quoted string is an error naming its key
// (common.go:1241-1280).
//
// shortcut: string-valued [Constants] entries are skipped -- no trigger
// compares strings yet -- and the character's localcoord, which Ikemen GO
// scales every default by (char.go:8123-8125) and Const240p by, is taken as
// the 320x240 default, since the .def's [Info] is not read here.
func ParseConstants(r io.Reader) (*Constants, error) {
	text, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("evaluator: reading constants: %w", err)
	}
	c := newDefaultConstants()
	seen := map[string]bool{}
	lines := strings.Split(strings.TrimPrefix(string(text), "\ufeff"), "\n")
	for i := 0; i < len(lines); {
		name, ok := sectionName(lines[i])
		i++
		if !ok {
			continue
		}
		section := map[string]string{}
		for ; i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "["); i++ {
			addKey(section, lines[i])
		}
		if seen[name] {
			continue
		}
		switch name {
		case "data":
			c.readData(section)
		case "size":
			c.readSize(section)
		case "velocity":
			c.readVelocity(section)
		case "movement":
			c.readKeys(section, movementKeys)
		case "constants":
			if err := c.readCustom(section); err != nil {
				return nil, err
			}
		default:
			continue
		}
		seen[name] = true
	}
	return c, nil
}

// sectionName returns the lower-case name of a "[Name]" line.
func sectionName(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[") {
		return "", false
	}
	end := strings.Index(line, "]")
	if end < 0 {
		return "", false
	}
	return strings.ToLower(strings.TrimSpace(line[1:end])), true
}

// addKey stores a "key = value" line's first occurrence of key, lower-cased
// (common.go:1071-1091 IniSection.Parse).
func addKey(section map[string]string, line string) {
	line, _, _ = strings.Cut(line, ";")
	line = strings.TrimSpace(line)
	end := strings.IndexAny(line, "= \t")
	if end <= 0 {
		return
	}
	key := strings.ToLower(line[:end])
	if _, exists := section[key]; exists {
		return
	}
	_, value, _ := strings.Cut(line, "=")
	section[key] = strings.TrimSpace(value)
}

// readKeys applies each key present in section to the names it fills. A value
// missing from a comma list keeps its default (common.go:1118-1132 ReadF32).
func (c *Constants) readKeys(section map[string]string, keys []constantKey) {
	for _, k := range keys {
		raw, ok := section[k.key]
		if !ok || raw == "" {
			continue
		}
		for i, field := range strings.Split(raw, ",") {
			if i >= len(k.names) {
				break
			}
			if field = strings.TrimSpace(field); field != "" {
				c.set(k.names[i], atof(field), k.ints)
			}
		}
	}
}

func (c *Constants) readData(section map[string]string) {
	c.readKeys(section, dataKeys[:1]) // life
	// dizzypoints and guardpoints default to the character's life.
	life := c.builtIn["data.life"].Number
	c.set("data.dizzypoints", life, true)
	c.set("data.guardpoints", life, true)
	c.readKeys(section, dataKeys[1:])
	for _, key := range []string{"intpersistindex", "floatpersistindex"} {
		// Present but empty reads 0 (char.go:4460-4468, Ikemen GO issue 2422).
		if raw, ok := section[key]; ok && raw == "" {
			c.set("data."+key, 0, true)
		}
	}
	// volume and volumescale convert with integer arithmetic (char.go:4453-4458).
	if raw, ok := section["volume"]; ok && raw != "" {
		c.set("data.volume", float64(int(atof(raw))/2+256), true)
	}
	if raw, ok := section["volumescale"]; ok && raw != "" {
		c.set("data.volume", float64(int(atof(raw))*64/25), true)
	}
	c.setDefenceMul()
}

// readSize reads the legacy ground/air/height keys, then the sizeboxes over
// them (char.go:4406-4430). A box is back, top, front, bottom: the back and
// the top are stored negated, so Const returns them positive.
func (c *Constants) readSize(section map[string]string) {
	c.readKeys(section, sizeKeys)
	c.readSizeBox(section["stand.sizebox"], "size.ground.back", "size.height", "size.ground.front")
	c.readSizeBox(section["air.sizebox"], "size.air.back", "", "size.air.front")
	c.setAttackDistAliases()
}

func (c *Constants) readSizeBox(raw, back, height, front string) {
	for i, field := range strings.Split(raw, ",") {
		if field = strings.TrimSpace(field); field == "" {
			continue
		}
		v := atof(field)
		switch {
		case i == 0:
			c.set(back, -v, false)
		case i == 1 && height != "":
			c.set(height, -v, false)
		case i == 2:
			c.set(front, v, false)
		}
	}
}

// readVelocity reads the jump velocities, makes the run and air jumps default
// to them, then reads the rest (char.go:4486-4504).
func (c *Constants) readVelocity(section map[string]string) {
	c.readKeys(section, velocityJumpKeys)
	for _, d := range [...]struct{ to, from string }{
		{"velocity.runjump.back.x", "velocity.jump.back.x"},
		{"velocity.runjump.back.y", "velocity.jump.y"},
		{"velocity.runjump.fwd.x", "velocity.jump.fwd.x"},
		{"velocity.runjump.y", "velocity.jump.y"},
		{"velocity.airjump.neu.x", "velocity.jump.neu.x"},
		{"velocity.airjump.y", "velocity.jump.y"},
		{"velocity.airjump.back.x", "velocity.jump.back.x"},
		{"velocity.airjump.fwd.x", "velocity.jump.fwd.x"},
	} {
		c.builtIn[d.to] = c.builtIn[d.from]
	}
	c.readKeys(section, velocityKeys)
}

// readCustom reads the [Constants] section, each value auto-typed as an int,
// a float32 or a quoted string (common.go:1241-1280). A key naming a built-in
// constant is ignored: Ikemen GO compiles those names to their own opcodes,
// never to this map.
func (c *Constants) readCustom(section map[string]string) error {
	for key, raw := range section {
		if _, builtIn := c.builtIn[key]; builtIn || strings.Contains(raw, `"`) {
			continue
		}
		if n, err := strconv.ParseInt(raw, 10, 32); err == nil {
			c.custom[key] = intValue(float64(n))
			continue
		}
		f, err := strconv.ParseFloat(raw, 32)
		if err != nil {
			return fmt.Errorf("evaluator: constants: [Constants] key %q: value %q must be a quoted string or a number", key, raw)
		}
		c.custom[key] = floatValue(f)
	}
	return nil
}

// atof reads the numeric prefix of s as Ikemen GO's Atof does
// (common.go:227), 0 when there is none.
func atof(s string) float64 {
	s = strings.TrimSpace(s)
	end := 0
	if end < len(s) && (s[end] == '-' || s[end] == '+') {
		end++
	}
	digits, dot := false, false
	for ; end < len(s); end++ {
		if s[end] == '.' && !dot {
			dot = true
			continue
		}
		if s[end] < '0' || s[end] > '9' {
			break
		}
		digits = true
	}
	if !digits {
		return 0
	}
	f, _ := strconv.ParseFloat(s[:end], 64)
	return f
}
