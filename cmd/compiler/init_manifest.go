package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"sec/internal/project"
)

// rejectNestedInitManifest prevents new project creation from enclosing an
// already initialized child project. Discovery is read-only and skips symlinks.
// Rules: rules/projects/projects.md — §§4(1–8),43(1–3).
func rejectNestedInitManifest(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == root && os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !entry.IsDir() && entry.Name() == "sec.toml" && filepath.Base(filepath.Dir(path)) == ".sec" {
			return fmt.Errorf("nested Sec project manifest is not allowed: existing child manifest %s", path)
		}
		return nil
	})
}

// discoverInitManifest uses the single upward-discovered project for invocation
// from a subdirectory, but rejects an actual nested manifest beneath another.
// Rules: rules/projects/projects.md — §§4,43(1),45.
func discoverInitManifest(start string) (string, bool, error) {
	manifest, ok := findNearestProjectManifest(start)
	if !ok {
		return "", false, nil
	}
	root := filepath.Dir(filepath.Dir(manifest))
	if parent, ok := findNearestProjectManifest(filepath.Dir(root)); ok && !samePath(parent, manifest) {
		return "", false, fmt.Errorf("nested Sec project manifest is not allowed: %s beneath %s", manifest, parent)
	}
	return manifest, true, nil
}

// readInitIdentity validates an existing manifest before any additive init writes.
// Rules: rules/projects/projects.md — §§7–11,§40(3),§43(1–2).
func readInitIdentity(manifest string) (project.Identity, error) {

	resolved, err := project.LoadManifest(manifest)
	if err != nil {
		return project.Identity{}, err
	}
	identity := resolved.Identity()
	for _, warning := range identity.Warnings {
		fmt.Fprintf(os.Stderr, "init warning: %s: %s\n", manifest, warning.Error())
	}

	return identity, nil
}
