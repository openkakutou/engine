package engine

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
)

// deniedFS is a file system whose file denied cannot be read.
type deniedFS struct {
	fstest.MapFS
	denied string
}

func (d deniedFS) Open(name string) (fs.File, error) {
	if name == d.denied {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return d.MapFS.Open(name)
}

func (d deniedFS) Stat(name string) (fs.FileInfo, error) {
	if name == d.denied {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrPermission}
	}
	return d.MapFS.Stat(name)
}

func (d deniedFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == d.denied {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrPermission}
	}
	return d.MapFS.ReadDir(name)
}

// A state file that exists but cannot be read is an error, not a missing
// file the character silently plays without.
func TestLoadFighterProgramFS_UnreadableStateFileIsAnError(t *testing.T) {
	fsys := deniedFS{
		MapFS:  fstest.MapFS{"chars/a/a.def": {}, "chars/a/common1.cns": {Data: []byte("[Statedef 0]\ntype = S\n")}},
		denied: "chars/a/common1.cns",
	}
	_, missing, err := LoadFighterProgramFS(fsys, stcommonChar("common1.cns"), "chars/a/a.def")
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("err = %v, missing = %v; want a permission error", err, missing)
	}
}

// A folder that cannot be listed while a name is matched without case is
// an error too.
func TestLoadFighterProgramFS_UnlistableFolderIsAnError(t *testing.T) {
	fsys := deniedFS{
		MapFS:  fstest.MapFS{"chars/a/a.def": {}, "chars/a/COMMON1.cns": {Data: []byte("[Statedef 0]\ntype = S\n")}},
		denied: "chars/a",
	}
	_, missing, err := LoadFighterProgramFS(fsys, stcommonChar("common1.cns"), "chars/a/a.def")
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("err = %v, missing = %v; want a permission error", err, missing)
	}
}
