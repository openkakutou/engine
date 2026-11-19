package conformance

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/openkakutou/sff"
)

// writeCharacter writes a minimal loadable synthetic character named base
// into dir: base.def (with filesLines appended to its [Files] section),
// base.cns, base.air, base.sff, plus every extra file (name -> content).
// No real character data is involved.
func writeCharacter(t *testing.T, dir, base, cnsSource, filesLines string, extra map[string]string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	write := func(name string, content []byte) {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o644); err != nil {
			t.Fatalf("test setup: writing %s: %v", name, err)
		}
	}
	def := "[Info]\nname = \"" + base + "\"\n\n[Files]\nsprite = " + base + ".sff\nanim = " + base + ".air\n" + filesLines + "\n"
	write(base+".def", []byte(def))
	if cnsSource != "" {
		write(base+".cns", []byte(cnsSource))
	}
	write(base+".air", []byte("[Begin Action 0]\n0,0, 0,0, -1\n"))

	pcx, err := sff.EncodePCX(&sff.PCXImage{Width: 2, Height: 2, Pixels: []byte{0, 1, 2, 3}})
	if err != nil {
		t.Fatalf("test setup: EncodePCX: %v", err)
	}
	var buf bytes.Buffer
	sprites := []sff.V1WriteSprite{{Group: 0, Image: 0, AxisX: 1, AxisY: 1, PixelData: pcx, Palette: make([]byte, sff.V1PaletteBlockSize)}}
	if err := sff.SerializeV1(&buf, [4]byte{1, 0, 0, 1}, false, sprites); err != nil {
		t.Fatalf("test setup: SerializeV1: %v", err)
	}
	write(base+".sff", buf.Bytes())
	for name, content := range extra {
		write(name, []byte(content))
	}
	return filepath.Join(dir, base+".def")
}

// exposureCNS has one state reached by the run (0) and one never reached
// (100). In state 0: controller A has an unsupported type and the unknown
// trigger foobartrigger; B is a supported VarSet using foobartrigger again
// and the unknown function bogus2; C is a supported VarSet whose trigger
// does not parse.
const exposureCNS = `[Statedef 0]
type = S
physics = S
ctrl = 1

[State 0, A]
type = NotARealController
trigger1 = foobartrigger > 0

[State 0, B]
type = VarSet
trigger1 = foobartrigger > 5 && bogus2(1) = 0
v = 0
value = 1

[State 0, C]
type = VarSet
trigger1 = 1 +* (
v = 0
value = 1

[Statedef 100]
type = S
physics = S

[State 100, D]
type = VarSet
trigger1 = 1
v = 0
value = 1
`

// tickErrorCNS fails at runtime: its ChangeState value is not an expression
// (a change to a missing state is no error: it enters an empty state).
const tickErrorCNS = `[Statedef 0]
type = S
physics = S
ctrl = 1

[State 0, Bad]
type = ChangeState
trigger1 = Time = 3
value = 999 +* (
`
