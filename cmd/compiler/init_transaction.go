package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type initPlannedFile struct {
	path string
	data []byte
}

// initPlan retains the complete create/preserve plan before any filesystem changes.
// Rules: rules/projects/projects.md — §§42–44.
type initPlan struct {
	directories        []string
	files              []initPlannedFile
	preserved          []initPlannedFile
	checkedDirectories map[string]os.FileInfo
	edits              []initManifestEdit
}

// addDirectory inspects all missing ancestors before adding ordered creates.
// Existing non-directory and symlink destinations are conflicts, never followed.
// Rules: rules/projects/projects.md — §§42,43(1–3).
func (plan *initPlan) addDirectory(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for _, known := range plan.directories {
		if known == path {
			return nil
		}
	}
	info, err := os.Lstat(path)
	if err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("init conflict: %s is not an ordinary directory", path)
		}
		if plan.checkedDirectories == nil {
			plan.checkedDirectories = map[string]os.FileInfo{}
		}
		plan.checkedDirectories[path] = info
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return err
	}
	if err := plan.addDirectory(parent); err != nil {
		return err
	}
	plan.directories = append(plan.directories, path)
	return nil
}

// addFile plans exclusive creates and snapshots compatible existing files. The
// caller must validate source compatibility before allowing preservation.
// Rules: rules/projects/projects.md — §§42,43(1–3).
func (plan *initPlan) addFile(path string, data []byte, compatible bool) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err := plan.addDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		plan.files = append(plan.files, initPlannedFile{path: path, data: data})
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("init conflict: %s is not an ordinary file", path)
	}
	existing, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(existing, data) && !compatible {
		return fmt.Errorf("init conflict: incompatible existing file %s", path)
	}
	plan.preserved = append(plan.preserved, initPlannedFile{path: path, data: existing})
	return nil
}

type initWritable interface {
	io.Writer
	Sync() error
	Close() error
	Stat() (os.FileInfo, error)
}
type initFileCreator func(string) (initWritable, error)

// createInitFile acquires a new path exclusively so concurrent user files survive.
// Rules: rules/projects/projects.md — §§42,43(4).
func createInitFile(path string) (initWritable, error) {
	return os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
}

// apply executes only a fully inspected plan. Exclusive file creation prevents
// overwrites; failures roll back owned files and empty directories in reverse
// order. Changed/replaced paths are preserved and rollback errors are reported.
// Rules: rules/projects/projects.md — §§42(2),43(2–4).
func (plan *initPlan) apply(create initFileCreator) error {
	return plan.applyWithEdits(create, replaceInitManifest)
}

// applyWithEdits commits manifest patches after source creates and rolls back
// edits before removing new source paths if any step fails.
// Rules: rules/projects/projects.md — §§41–43.
func (plan *initPlan) applyWithEdits(create initFileCreator, replace initFileReplacer) (err error) {
	type ownedPath struct {
		path string
		info os.FileInfo
	}
	var owned []ownedPath
	var undoEdits []func() error
	defer func() {
		if err == nil {
			return
		}
		for i := len(undoEdits) - 1; i >= 0; i-- {
			if undoErr := undoEdits[i](); undoErr != nil {
				err = errors.Join(err, undoErr)
			}
		}
		for i := len(owned) - 1; i >= 0; i-- {
			path := owned[i]
			current, statErr := os.Lstat(path.path)
			if os.IsNotExist(statErr) {
				continue
			}
			if statErr != nil {
				err = errors.Join(err, fmt.Errorf("rollback inspect %s: %w", path.path, statErr))
				continue
			}
			if path.info == nil || !os.SameFile(path.info, current) {
				err = errors.Join(err, fmt.Errorf("rollback preserved replaced path %s", path.path))
				continue
			}
			if removeErr := os.Remove(path.path); removeErr != nil {
				err = errors.Join(err, fmt.Errorf("rollback %s: %w", path.path, removeErr))
			}
		}
	}()
	checked := make([]string, 0, len(plan.checkedDirectories))
	for path := range plan.checkedDirectories {
		checked = append(checked, path)
	}
	sort.Strings(checked)
	for _, path := range checked {
		expected := plan.checkedDirectories[path]
		current, statErr := os.Lstat(path)
		if statErr != nil || !current.IsDir() || current.Mode()&os.ModeSymlink != 0 || !os.SameFile(current, expected) {
			return fmt.Errorf("init conflict: inspected directory changed: %s", path)
		}
	}
	// A plan must not silently accept a user edit between inspection and apply.
	for _, file := range plan.preserved {
		info, statErr := os.Lstat(file.path)
		if statErr != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("init conflict: preserved file changed: %s", file.path)
		}
		current, readErr := os.ReadFile(file.path)
		if readErr != nil {
			return readErr
		}
		if !bytes.Equal(current, file.data) {
			return fmt.Errorf("init conflict: preserved file changed: %s", file.path)
		}
	}
	for _, path := range plan.directories {
		if mkdirErr := os.Mkdir(path, 0755); mkdirErr != nil {
			if !os.IsExist(mkdirErr) {
				return mkdirErr
			}
			info, statErr := os.Lstat(path)
			if statErr != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("init conflict: directory changed: %s", path)
			}
			continue
		}
		info, statErr := os.Lstat(path)
		owned = append(owned, ownedPath{path, info})
		if statErr != nil {
			return statErr
		}
	}
	for _, file := range plan.files {
		output, openErr := create(file.path)
		if openErr != nil {
			return fmt.Errorf("create %s: %w", file.path, openErr)
		}
		info, statErr := output.Stat()
		owned = append(owned, ownedPath{file.path, info})
		if statErr != nil {
			return errors.Join(statErr, output.Close())
		}
		count, writeErr := output.Write(file.data)
		if writeErr == nil && count != len(file.data) {
			writeErr = io.ErrShortWrite
		}
		if writeErr == nil {
			writeErr = output.Sync()
		}
		closeErr := output.Close()
		if writeErr != nil || closeErr != nil {
			return fmt.Errorf("write %s: %w", file.path, errors.Join(writeErr, closeErr))
		}
	}
	for _, edit := range plan.edits {
		undo, editErr := replace(edit)
		if undo != nil {
			undoEdits = append(undoEdits, undo)
		}
		if editErr != nil {
			return fmt.Errorf("edit manifest %s: %w", edit.path, editErr)
		}
	}

	return nil
}

// executeInitPlan shares validation/planning between dry run and real execution.
// Rules: rules/projects/projects.md — §§43,44.
func executeInitPlan(plan *initPlan, dryRun bool) error {
	if !dryRun {
		return plan.apply(createInitFile)
	}
	for _, path := range plan.directories {
		fmt.Printf("create directory %s\n", path)
	}
	for _, file := range plan.files {
		fmt.Printf("create file %s\n", file.path)
	}
	for _, file := range plan.preserved {
		fmt.Printf("preserve file %s\n", file.path)
	}
	for _, edit := range plan.edits {
		fmt.Printf("edit manifest %s\n", edit.path)
	}
	if len(plan.directories) == 0 && len(plan.files) == 0 && len(plan.edits) == 0 {
		fmt.Println("no changes")
	}
	return nil
}
