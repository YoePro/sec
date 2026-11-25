package cbindings

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// ErrStaleFile means the last pre-replacement hash no longer matches.
var ErrStaleFile = errors.New("the file changed after it was read")
var fileMutationMu sync.Mutex

// SHA256 identifies raw file bytes, including comments and whitespace.
// Rules: MD-021 correction §5(1)–(3).
func SHA256(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// ReplaceFileAtomically shares the owning CLI's same-directory temporary-file
// replacement, permission preservation and final optimistic hash check.
// Cooperating in-process callers serialize their final check and rename.
// Independent writers must follow the same optimistic protocol; the host
// filesystem supplies atomic replacement, not a cross-process compare/swap.
// Rules: MD-021 correction §5(2)–(3); harness design notes §5(3)–(4).
func ReplaceFileAtomically(file string, data []byte, expected *string) error {
	return replaceFileUsing(nil, file, data, expected)
}

// replaceFileUsing uses the same preparation/check/rename algorithm under an
// optional os.Root. All rooted path operations stay in that OS-enforced tree;
// temporary permission changes use the opened file handle, never a path race.
// Rules: harness design notes §§5(4),5(6); MD-021 correction §5(2)–(3).
func replaceFileUsing(root *os.Root, file string, data []byte, expected *string) error {
	mkdirAll, stat, lstat := os.MkdirAll, os.Stat, os.Lstat
	readFile, rename, remove := os.ReadFile, os.Rename, os.Remove
	createTemp := func(directory string) (*os.File, string, error) {
		handle, err := os.CreateTemp(directory, ".c-bindings-*.toml")
		if err != nil {
			return nil, "", err
		}
		return handle, handle.Name(), nil
	}
	if root != nil {
		mkdirAll, stat, lstat = root.MkdirAll, root.Stat, root.Lstat
		readFile, rename, remove = root.ReadFile, root.Rename, root.Remove
		createTemp = func(directory string) (*os.File, string, error) {
			for range 10 {
				name := filepath.Join(directory, ".c-bindings-"+rand.Text()+".toml")
				handle, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
				if os.IsExist(err) {
					continue
				}
				return handle, name, err
			}
			return nil, "", fs.ErrExist
		}
	}
	fileMutationMu.Lock()
	defer fileMutationMu.Unlock()
	if err := mkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if info, err := stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	temporary, name, err := createTemp(filepath.Dir(file))
	if err != nil {
		return err
	}
	defer remove(name)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(mode); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if expected != nil {
		current, err := readFile(file)
		if err != nil || SHA256(current) != *expected {
			return ErrStaleFile
		}
	} else if _, err := lstat(file); !os.IsNotExist(err) {
		return ErrStaleFile
	}
	return rename(name, file)
}
