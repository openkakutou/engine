package conformance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openkakutou/character"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/character/zss"
	engine "github.com/openkakutou/engine"
)

// buildProgram assembles the state table of a loaded character the way
// Ikemen GO does and returns it as the engine's FighterProgram: it compiles
// the state files in this order -- every "st" and "stN" .def entry (sorted:
// st, then st1, st2, ...), then the .cmd file (its Statedef -1/-2/-3 live
// there), then "stcommon" -- and the first definition of a state number wins,
// whichever the format (.cns or .zss), so a character's own states override
// its common file's (Ikemen GO src/compiler.go:8776-8798 for the order,
// :7538-7539 for "skip appending if already declared"; commit 54d4148a). A
// number defined twice inside one file is likewise first-wins (:7248).
//
// character.Load only parses the .def's "cns" file into Character.StateDefs
// and discards the other .cns state files, so they are parsed here; the .zss
// files are in Character.Scripts, already parsed, matched to their entry by
// file name (a non-.zss entry that does not exist but has an "<entry>.zss"
// is the one character.Load loaded, Ikemen GO stateCompile :7181).
// Character.StateDefs comes last, as a fallback: Ikemen GO reads "cns" for
// constants only, but MUGEN 1.0 characters commonly keep states there.
//
// A state file that does not exist is skipped (stcommon usually names the
// engine's own common1.cns, which a character folder does not hold). A .zss
// file that character.Load could not parse is an error naming it.
//
// shortcut: Ikemen GO merges the controllers of a negative state defined in
// several files for characters flagged ikemenversion (:7269, :7538) and for
// every .zss (:8466); here the first file's negative state wins. Upgrade if
// a real character's -1 split across files shows up in the results. The
// "data/" search directory of LoadFile is not searched either.
func buildProgram(ch *character.Character, defPath string) (engine.FighterProgram, error) {
	dir := filepath.Dir(defPath)
	entries := append([]string{}, ch.StateFiles...)
	entries = append(entries, ch.CommandFile, ch.StateCommonFile)
	scripts := map[string]zss.Script{}
	for _, script := range ch.Scripts {
		// shortcut: character.Load gives no typed error, so a file that is
		// not there is told from a parse failure by its message; upgrade when
		// it exports one. A missing .zss is skipped like a missing .cns (the
		// common file lives in Ikemen GO's data/, which is not searched).
		if strings.Contains(script.Error, "opening state script file") {
			continue
		}
		if script.Error != "" {
			return engine.FighterProgram{}, fmt.Errorf("state script %q: %s", script.File, script.Error)
		}
		scripts[strings.ToLower(script.File)] = script.Script
	}
	// The .cmd file's states are the ones character.Load already parsed with
	// cmd.Parse (which synthesizes a missing "[Statedef -1]" header, character
	// ADR 026), not a second cns.Parse that would reject such a file.

	var files []engine.StateFile
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		if entry == ch.CommandFile {
			files = append(files, engine.StateFile{States: ch.Commands.States})
			continue
		}
		if script, ok := scripts[strings.ToLower(entry)]; ok && strings.EqualFold(filepath.Ext(entry), ".zss") {
			files = append(files, engine.StateFile{Script: &script})
			continue
		}
		path, ok := resolveCaseInsensitive(dir, strings.ReplaceAll(entry, "\\", "/"))
		if !ok {
			if script, found := scripts[strings.ToLower(entry+".zss")]; found {
				files = append(files, engine.StateFile{Script: &script})
			}
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return engine.FighterProgram{}, fmt.Errorf("opening state file %q: %w", entry, err)
		}
		defs, err := cns.Parse(f)
		f.Close()
		if err != nil {
			return engine.FighterProgram{}, fmt.Errorf("parsing state file %q: %w", entry, err)
		}
		files = append(files, engine.StateFile{States: defs})
	}
	files = append(files, engine.StateFile{States: ch.StateDefs})
	return engine.NewFighterProgramFromFiles(files, ch.Animations, ch.Commands), nil
}

// resolveCaseInsensitive resolves relPath under base one component at a time,
// matching each component case-insensitively when no exact match exists, as
// real .def files are written for case-insensitive Windows filesystems. It
// reports false when a component has no match.
func resolveCaseInsensitive(base, relPath string) (string, bool) {
	current := base
	for _, component := range strings.Split(relPath, "/") {
		if component == "" || component == "." {
			continue
		}
		candidate := filepath.Join(current, component)
		if _, err := os.Stat(candidate); err == nil {
			current = candidate
			continue
		}
		entries, err := os.ReadDir(current)
		if err != nil {
			return "", false
		}
		match := ""
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), component) {
				match = entry.Name()
				break
			}
		}
		if match == "" {
			return "", false
		}
		current = filepath.Join(current, match)
	}
	return current, true
}
