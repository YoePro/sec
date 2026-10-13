package project

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ReplaceManifestSnapshot stages a same-directory, permission-preserving atomic
// replacement. Original bytes and file identity are rechecked before rename;
// undo restores the snapshot only while the committed file is still owned.
// A byte-identical edit validates its snapshot without rewriting the file.
// Callers serialize cooperating edits and own the transaction success policy.
// Rules: rules/projects/projects.md — §§29(6–12),41,43(4),49(5).
func ReplaceManifestSnapshot(path string, before, after []byte) (func() error, error) {
	original, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !original.Mode().IsRegular() {
		return nil, fmt.Errorf("manifest changed: %s", path)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(current, before) {
		return nil, fmt.Errorf("manifest changed: %s", path)
	}
	if bytes.Equal(before, after) {
		return func() error { return nil }, nil
	}
	staged, err := os.CreateTemp(filepath.Dir(path), ".sec-manifest-*")
	if err != nil {
		return nil, err
	}
	temporary := staged.Name()
	defer os.Remove(temporary)
	writeErr := staged.Chmod(original.Mode().Perm())
	if writeErr == nil {
		_, writeErr = staged.Write(after)
	}
	if writeErr == nil {
		writeErr = staged.Sync()
	}
	closeErr := staged.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return nil, err
	}
	currentInfo, err := os.Lstat(path)
	if err != nil || !os.SameFile(original, currentInfo) {
		return nil, fmt.Errorf("manifest replaced during edit: %s", path)
	}
	current, err = os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(current, before) {
		return nil, fmt.Errorf("manifest changed during edit: %s", path)
	}
	// Record staged ownership before rename; rollback never removes a replacement.
	committed, err := os.Stat(temporary)
	if err != nil {
		return nil, err
	}
	if err := os.Rename(temporary, path); err != nil {
		return nil, err
	}
	undo := func() error {
		info, err := os.Lstat(path)
		if err != nil || !os.SameFile(info, committed) {
			return fmt.Errorf("rollback preserved replaced manifest %s", path)
		}
		_, err = ReplaceManifestSnapshot(path, after, before)
		return err
	}
	return undo, nil
}
