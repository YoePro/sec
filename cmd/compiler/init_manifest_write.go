package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"sec/internal/project"
)

type initManifestEdit struct {
	path          string
	before, after []byte
}
type initFileReplacer func(initManifestEdit) (func() error, error)

// addManifestEdit plans a precise replacement only after checking original bytes.
// Rules: rules/projects/projects.md — §§5(4–6),41,43,49(5).
func (plan *initPlan) addManifestEdit(path string, before, after []byte) error {
	if bytes.Equal(before, after) {
		return nil
	}
	if err := plan.addDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("init conflict: manifest %s is not an ordinary file", path)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(current, before) {
		return fmt.Errorf("init conflict: manifest changed: %s", path)
	}
	plan.edits = append(plan.edits, initManifestEdit{path: path, before: before, after: after})
	return nil
}

// replaceInitManifest stages the complete edited document in the same directory,
// preserves mode, rechecks the original and commits with atomic rename. The
// returned undo restores original bytes only while the committed file is owned.
// Rules: rules/projects/projects.md — §§41,43(4),49(5).
func replaceInitManifest(edit initManifestEdit) (func() error, error) {
	return project.ReplaceManifestSnapshot(edit.path, edit.before, edit.after)
}

// applyInitTarget serializes cooperating target additions, then executes the
// preflighted plan. Dry run never acquires a lock or stages a replacement.
// Rules: rules/projects/projects.md — §§40–44.
func applyInitTarget(plan *initPlan, manifest string, dryRun bool) error {
	if dryRun {
		return executeInitPlan(plan, true)
	}
	if len(plan.directories) == 0 && len(plan.files) == 0 && len(plan.edits) == 0 {
		return plan.apply(createInitFile)
	}
	lockPath := filepath.Join(filepath.Dir(manifest), ".init.lock")
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("cannot acquire init transaction lock %s: %w", lockPath, err)
	}
	defer os.Remove(lockPath)
	if err := lock.Close(); err != nil {
		return err
	}
	return plan.apply(createInitFile)
}
