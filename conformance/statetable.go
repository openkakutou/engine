package conformance

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openkakutou/character"
	"github.com/openkakutou/character/cns"
)

// buildStateTable assembles the state table of a loaded character the way
// Ikemen GO does: it compiles the state files in this order -- every "st"
// and "stN" .def entry (sorted: st, then st1, st2, ...), then the .cmd file
// (its Statedef -1/-2/-3 live there), then "stcommon" -- and the first
// definition of a state number wins, so a character's own states override
// its common file's (Ikemen GO src/compiler.go:8776-8798 for the order,
// :7538-7539 for "skip appending if already declared"; commit 54d4148a).
// A number defined twice inside one file is likewise first-wins (:7248).
//
// character.Load only parses the .def's "cns" file into Character.StateDefs
// and discards the other .cns state files, so they are parsed here.
// Character.StateDefs comes last, as a fallback: Ikemen GO reads "cns" for
// constants only, but MUGEN 1.0 characters commonly keep states there.
//
// A state file that does not exist is skipped (stcommon usually names the
// engine's own common1.cns, which a character folder does not hold). A .zss
// entry is skipped: Character.Scripts holds it and Tick does not run it.
//
// shortcut: Ikemen GO merges the controllers of a negative state defined in
// several files for characters flagged ikemenversion (:7269, :7538); here
// the first file's negative state wins as for MUGEN characters. Upgrade if
// a real character's -1 split across files shows up in the results. The
// "data/" search directory of LoadFile is not searched either.
func buildStateTable(ch *character.Character, defPath string) (map[int]cns.StateDef, error) {
	dir := filepath.Dir(defPath)
	entries := append([]string{}, ch.StateFiles...)
	entries = append(entries, ch.CommandFile, ch.StateCommonFile)
	// The .cmd file's states are the ones character.Load already parsed with
	// cmd.Parse (which synthesizes a missing "[Statedef -1]" header, character
	// ADR 026), not a second cns.Parse that would reject such a file.

	table := map[int]cns.StateDef{}
	add := func(defs []cns.StateDef) {
		for _, def := range defs {
			if _, exists := table[def.Number]; !exists {
				table[def.Number] = def
			}
		}
	}
	for _, entry := range entries {
		if entry != "" && entry == ch.CommandFile {
			add(ch.Commands.States)
			continue
		}
		if entry == "" || strings.EqualFold(filepath.Ext(entry), ".zss") {
			continue
		}
		path, ok := resolveCaseInsensitive(dir, strings.ReplaceAll(entry, "\\", "/"))
		if !ok {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("opening state file %q: %w", entry, err)
		}
		defs, err := cns.Parse(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("parsing state file %q: %w", entry, err)
		}
		add(defs)
	}
	add(ch.StateDefs)
	return table, nil
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
