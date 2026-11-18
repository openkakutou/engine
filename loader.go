package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openkakutou/character"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/evaluator"
)

// LoadFighterProgram assembles the state table of a loaded character the way
// Ikemen GO does and returns it as a FighterProgram: it compiles the state
// files in this order -- every "st" and "stN" .def entry (sorted: st, then
// st1, st2, ...), then the .cmd file (its Statedef -1/-2/-3 live there), then
// "stcommon" -- and the first definition of a state number wins, whichever
// the format (.cns or .zss), so a character's own states override its common
// file's (Ikemen GO src/compiler.go:8776-8798 for the order, :7538-7539 for
// "skip appending if already declared"; commit 54d4148a). A number defined
// twice inside one file is likewise first-wins (:7248).
//
// A state file is searched in the .def's folder first, then in each
// searchDir in order, case-insensitively: Ikemen GO's SearchFile/LoadFile
// (src/common.go ~720-775) look in the dirs given by the caller, and the
// compiler passes []string{def, "", sys.motif.Def, "data/"} (compiler.go:8793).
// The caller supplies the game's own folders (such as data/) as searchDirs;
// stcommon usually names the engine's own common1.cns, which a character
// folder does not hold. The first folder holding the file wins, as a whole:
// files are never merged across folders.
//
// .zss files are read and parsed here, from whichever folder holds them (the
// character.Character.Scripts only cover the .def's folder). A non-.zss entry
// that does not exist but has an "<entry>.zss" is loaded as that script
// (Ikemen GO stateCompile :7181). The .cmd file's states come from
// ch.Commands, already parsed by character.Load with cmd.Parse (which
// synthesizes a missing "[Statedef -1]" header, character ADR 026).
// ch.StateDefs (the .def's "cns" file) comes last, as a fallback: Ikemen GO
// reads "cns" for constants only, but MUGEN 1.0 characters commonly keep
// states there.
//
// missing lists the entries found in no folder, in load order: they are
// skipped, not an error, and the caller decides whether to report them. An
// unreadable file or one that fails to parse is an error naming it.
//
// shortcut: Ikemen GO merges the controllers of a negative state defined in
// several files for characters flagged ikemenversion (:7269, :7538) and for
// every .zss (:8466); here the first file's negative state wins. Upgrade if
// a real character's -1 split across files shows up in the results.
func LoadFighterProgram(ch *character.Character, defPath string, searchDirs ...string) (prog FighterProgram, missing []string, err error) {
	dirs := append([]string{filepath.Dir(defPath)}, searchDirs...)
	entries := append([]string{}, ch.StateFiles...)
	entries = append(entries, ch.CommandFile, ch.StateCommonFile)

	var files []StateFile
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		if entry == ch.CommandFile {
			files = append(files, StateFile{States: ch.Commands.States})
			continue
		}
		file, found, err := loadStateFile(dirs, entry)
		if err != nil {
			return FighterProgram{}, nil, err
		}
		if !found {
			missing = append(missing, entry)
			continue
		}
		files = append(files, file)
	}
	files = append(files, StateFile{States: ch.StateDefs})
	prog = NewFighterProgramFromFiles(files, ch.Animations, ch.Commands)
	prog.Constants, err = loadConstants(dirs, ch.ConstantsFile)
	if err != nil {
		return FighterProgram{}, nil, err
	}
	if prog.Constants == nil && ch.ConstantsFile != "" {
		missing = append(missing, ch.ConstantsFile)
	}
	return prog, missing, nil
}

// loadConstants reads the character's constants file, the .def's "cns"
// entry, searched in the same folders as a state file (Ikemen GO
// src/char.go:4403 LoadFile dirs {def, "", "data/"}). It returns nil when the
// entry is empty or found in no folder, and the character keeps Ikemen GO's
// defaults.
func loadConstants(dirs []string, entry string) (*evaluator.Constants, error) {
	if entry == "" {
		return nil, nil
	}
	path, ok := searchFile(dirs, strings.ReplaceAll(entry, "\\", "/"))
	if !ok {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening constants file %q: %w", entry, err)
	}
	defer f.Close()
	constants, err := evaluator.ParseConstants(f)
	if err != nil {
		return nil, fmt.Errorf("constants file %q: %w", path, err)
	}
	return constants, nil
}

// loadStateFile finds entry in dirs, falling back to "<entry>.zss", and
// parses it by its extension.
func loadStateFile(dirs []string, entry string) (StateFile, bool, error) {
	relPath := strings.ReplaceAll(entry, "\\", "/")
	path, ok := searchFile(dirs, relPath)
	if !ok && !strings.EqualFold(filepath.Ext(relPath), ".zss") {
		path, ok = searchFile(dirs, relPath+".zss")
	}
	if !ok {
		return StateFile{}, false, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return StateFile{}, false, fmt.Errorf("opening state file %q: %w", entry, err)
	}
	defer f.Close()
	if strings.EqualFold(filepath.Ext(path), ".zss") {
		script, err := zss.Parse(f)
		if err != nil {
			return StateFile{}, false, fmt.Errorf("state script %q: %w", path, err)
		}
		return StateFile{Script: &script}, true, nil
	}
	defs, err := cns.Parse(f)
	if err != nil {
		return StateFile{}, false, fmt.Errorf("parsing state file %q: %w", entry, err)
	}
	return StateFile{States: defs}, true, nil
}

// searchFile returns the first dir's match for relPath, in dirs order.
func searchFile(dirs []string, relPath string) (string, bool) {
	for _, dir := range dirs {
		if path, ok := resolveCaseInsensitive(dir, relPath); ok {
			return path, true
		}
	}
	return "", false
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
