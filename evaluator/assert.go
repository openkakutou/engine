package evaluator

import (
	"fmt"
	"strings"
)

// assertFlagNames are every AssertSpecial flag Ikemen GO accepts
// (compiler_functions.go:120-300, commit 54d4148a), in its order; a flag's
// bit is its index.
var assertFlagNames = [...]string{
	"intro", "invisible", "noairguard", "noautoturn", "nocrouchguard", "nojugglecheck", "noshadow",
	"nostandguard", "nowalk", "unguardable", "globalnoshadow", "nobardisplay", "nobg", "nofg", "noko",
	"nokoslow", "nokosnd", "nomusic", "roundnotover", "timerfreeze", "animatehitpause", "animfreeze",
	"autoguard", "drawunder", "noaibuttonjam", "noaicheat", "noailevel", "noairjump", "nobrake",
	"nocombodisplay", "nocornerpush", "nocrouch", "nodizzypointsdamage", "nofacedisplay", "nofacep2",
	"nofallcount", "nofalldefenceup", "nofallhitflag", "nofastrecoverfromliedown", "nogetupfromliedown",
	"noguardbardisplay", "noguarddamage", "noguardko", "noguardpointsdamage", "nohardcodedkeys",
	"nohitdamage", "noinput", "nointroreset", "nojump", "nokofall", "nokovelocity", "nolifebaraction",
	"nolifebardisplay", "nomakedust", "nonamedisplay", "nopowerbardisplay", "noredlifedamage", "noscore",
	"nostand", "nostunbardisplay", "noturntarget", "nowinicondisplay", "postroundinput",
	"projtypecollision", "runfirst", "runlast", "sizepushonly", "nodestroyself", "camerafreeze",
	"notimedisplay", "globalnoko", "roundnotskip", "roundfreeze", "skipfightdisplay", "skipkodisplay",
	"skiprounddisplay", "skipwindisplay",
}

// AssertFlags is the set of AssertSpecial flags a fighter's states asserted
// (Ikemen GO c.assertFlag and sys.specialFlag), one bit per assertFlagNames
// entry.
type AssertFlags [2]uint64

// assertFlagIndex is name's bit (any case), ok false for a flag Ikemen GO
// does not know.
func assertFlagIndex(name string) (int, bool) {
	for i, n := range assertFlagNames {
		if strings.EqualFold(strings.TrimSpace(name), n) {
			return i, true
		}
	}
	return 0, false
}

// Set asserts the flag name (on true) or clears it; an unknown name is an
// error, "Invalid AssertSpecial flag" in Ikemen GO.
func (f *AssertFlags) Set(name string, on bool) error {
	i, ok := assertFlagIndex(name)
	if !ok {
		return fmt.Errorf("invalid AssertSpecial flag %q", name)
	}
	bit := uint64(1) << (i % 64)
	if on {
		f[i/64] |= bit
		return nil
	}
	f[i/64] &^= bit
	return nil
}

// Has reports whether the flag name (any case) is asserted.
func (f AssertFlags) Has(name string) bool {
	i, ok := assertFlagIndex(name)
	return ok && f[i/64]&(uint64(1)<<(i%64)) != 0
}
