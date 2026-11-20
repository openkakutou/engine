package engine

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/openkakutou/character"
	"github.com/openkakutou/character/cns"
	"github.com/openkakutou/character/zss"
	"github.com/openkakutou/engine/v2/evaluator"
)

// LoadFighterProgramFS assembles the state table of a loaded character the way
// Ikemen GO does and returns it as a FighterProgram: it compiles the state
// files in this order -- every "st" and "stN" .def entry (sorted: st, then
// st1, st2, ...), then the .cmd file (its Statedef -1/-2/-3 live there), then
// "stcommon" -- and the first definition of a state number wins, whichever
// the format (.cns or .zss), so a character's own states override its common
// file's (Ikemen GO src/compiler.go:8776-8798 for the order, :7538-7539 for
// "skip appending if already declared"; commit 54d4148a). A number defined
// twice inside one file is likewise first-wins (:7248).
//
// defName and searchDirs are slash-separated paths inside fsys (backslashes
// are accepted); a path leaving the root of fsys is not found. A state file is
// searched in the .def's folder first, then in each searchDir in order, case-insensitively: Ikemen GO's SearchFile/LoadFile
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
func LoadFighterProgramFS(fsys fs.FS, ch *character.Character, defName string, searchDirs ...string) (prog FighterProgram, missing []string, err error) {
	dirs := []string{path.Dir(slashed(defName))}
	for _, dir := range searchDirs {
		dirs = append(dirs, slashed(dir))
	}
	return assembleFighterProgram(fsAccess{fsys}, ch, dirs)
}

// LoadFighterProgram is LoadFighterProgramFS over the disk: defPath and
// searchDirs are OS paths, used as the caller wrote them (relative ones are
// resolved by the OS from the working directory), and its errors name files
// with those paths.
func LoadFighterProgram(ch *character.Character, defPath string, searchDirs ...string) (prog FighterProgram, missing []string, err error) {
	dirs := append([]string{filepath.Dir(defPath)}, searchDirs...)
	return assembleFighterProgram(osAccess{}, ch, dirs)
}

// fileAccess is what the search needs of a file tree, so that one resolution
// logic serves both an fs.FS (path semantics, a path leaving the root is not
// found) and the OS (the caller's own paths, joined with filepath.Join).
type fileAccess interface {
	// join appends one path component to dir; false when the result is not
	// a name this tree can hold.
	join(dir, component string) (string, bool)
	exists(name string) bool
	// readDir lists the entry names of the directory name.
	readDir(name string) ([]string, error)
	open(name string) (io.ReadCloser, error)
}

type fsAccess struct{ fsys fs.FS }

func (a fsAccess) join(dir, component string) (string, bool) {
	name := path.Join(dir, component)
	return name, fs.ValidPath(name)
}

func (a fsAccess) exists(name string) bool {
	_, err := fs.Stat(a.fsys, name)
	return err == nil
}

func (a fsAccess) readDir(name string) ([]string, error) {
	entries, err := fs.ReadDir(a.fsys, name)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

func (a fsAccess) open(name string) (io.ReadCloser, error) {
	return a.fsys.Open(name)
}

type osAccess struct{}

func (osAccess) join(dir, component string) (string, bool) {
	return filepath.Join(dir, component), true
}

func (osAccess) exists(name string) bool {
	_, err := os.Stat(name)
	return err == nil
}

func (osAccess) readDir(name string) ([]string, error) {
	entries, err := os.ReadDir(name)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names, nil
}

func (osAccess) open(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

// assembleFighterProgram is the one implementation behind both loaders; dirs is
// the .def's folder followed by the search folders.
func assembleFighterProgram(files fileAccess, ch *character.Character, dirs []string) (prog FighterProgram, missing []string, err error) {
	entries := append([]string{}, ch.StateFiles...)
	entries = append(entries, ch.CommandFile, ch.StateCommonFile)

	var stateFiles []StateFile
	for _, entry := range entries {
		if entry == "" {
			continue
		}
		if entry == ch.CommandFile {
			stateFiles = append(stateFiles, StateFile{States: ch.Commands.States})
			continue
		}
		file, found, err := loadStateFile(files, dirs, entry)
		if err != nil {
			return FighterProgram{}, nil, err
		}
		if !found {
			missing = append(missing, entry)
			continue
		}
		stateFiles = append(stateFiles, file)
	}
	stateFiles = append(stateFiles, StateFile{States: ch.StateDefs})
	prog = NewFighterProgramFromFiles(stateFiles, ch.Animations, ch.Commands)
	prog.IkemenVersion = parseIkemenVersion(ch.IkemenVersion)
	prog.Constants, err = loadConstants(files, dirs, ch.ConstantsFile)
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
func loadConstants(files fileAccess, dirs []string, entry string) (*evaluator.Constants, error) {
	if entry == "" {
		return nil, nil
	}
	name, ok := searchFile(files, dirs, slashed(entry))
	if !ok {
		return nil, nil
	}
	f, err := files.open(name)
	if err != nil {
		return nil, fmt.Errorf("opening constants file %q: %w", entry, err)
	}
	defer f.Close()
	constants, err := evaluator.ParseConstants(f)
	if err != nil {
		return nil, fmt.Errorf("constants file %q: %w", name, err)
	}
	return constants, nil
}

// loadStateFile finds entry in dirs, falling back to "<entry>.zss", and
// parses it by its extension.
func loadStateFile(files fileAccess, dirs []string, entry string) (StateFile, bool, error) {
	relPath := slashed(entry)
	name, ok := searchFile(files, dirs, relPath)
	if !ok && !strings.EqualFold(filepath.Ext(relPath), ".zss") {
		name, ok = searchFile(files, dirs, relPath+".zss")
	}
	if !ok {
		return StateFile{}, false, nil
	}
	f, err := files.open(name)
	if err != nil {
		return StateFile{}, false, fmt.Errorf("opening state file %q: %w", entry, err)
	}
	defer f.Close()
	if strings.EqualFold(filepath.Ext(name), ".zss") {
		script, err := zss.Parse(f)
		if err != nil {
			return StateFile{}, false, fmt.Errorf("state script %q: %w", name, err)
		}
		return StateFile{Script: &script}, true, nil
	}
	defs, err := cns.Parse(f)
	if err != nil {
		return StateFile{}, false, fmt.Errorf("parsing state file %q: %w", entry, err)
	}
	return StateFile{States: defs}, true, nil
}

// slashed turns the backslashes of a name written for Windows into slashes.
func slashed(name string) string {
	return strings.ReplaceAll(name, "\\", "/")
}

// searchFile returns the first dir's match for relPath, in dirs order.
func searchFile(files fileAccess, dirs []string, relPath string) (string, bool) {
	for _, dir := range dirs {
		if name, ok := resolveCaseInsensitive(files, dir, relPath); ok {
			return name, true
		}
	}
	return "", false
}

// resolveCaseInsensitive resolves relPath under base one component at a time,
// matching each component case-insensitively when no exact match exists, as
// real .def files are written for case-insensitive Windows filesystems. It
// reports false when a component has no match.
func resolveCaseInsensitive(files fileAccess, base, relPath string) (string, bool) {
	current := base
	for _, component := range strings.Split(relPath, "/") {
		if component == "" || component == "." {
			continue
		}
		candidate, ok := files.join(current, component)
		if !ok {
			return "", false
		}
		if files.exists(candidate) {
			current = candidate
			continue
		}
		names, err := files.readDir(current)
		if err != nil {
			return "", false
		}
		match := ""
		for _, name := range names {
			if strings.EqualFold(name, component) {
				match = name
				break
			}
		}
		if match == "" {
			return "", false
		}
		current, ok = files.join(current, match)
		if !ok {
			return "", false
		}
	}
	return current, true
}

// parseIkemenVersion is Ikemen GO's ParseIkemenVersion (common.go:931-945):
// up to three dot-separated numbers, stopping at the first that is not one.
func parseIkemenVersion(raw string) [3]uint16 {
	var ver [3]uint16
	for i, part := range strings.Split(raw, ".") {
		if i >= len(ver) {
			break
		}
		v, err := strconv.ParseUint(strings.TrimSpace(part), 10, 16)
		if err != nil {
			break
		}
		ver[i] = uint16(v)
	}
	return ver
}
