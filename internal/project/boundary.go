package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// BoundaryViolation locates a nested manifest and its outer governing manifest.
// Rules: rules/projects/projects.md — §§4(1–3),48(1–2).
type BoundaryViolation struct {
	Manifest, GoverningManifest string
	Line                        int
}

// ValidateBoundary discovers every nested manifest beneath the outermost
// governing project, including unsaved overlays. Directory symlinks are not
// followed, matching init discovery; I/O errors are returned, never treated as
// proof that a project has no nested manifests. The operation is read-only.
// Rules: rules/projects/projects.md — §§4,36,48(1–3).
func ValidateBoundary(manifest string, overlays map[string]string) ([]BoundaryViolation, error) {
	manifest, err := filepath.Abs(manifest)
	if err != nil {
		return nil, err
	}
	manifest = filepath.Clean(manifest)
	snapshots := map[string]string{}
	for path, text := range overlays {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		snapshots[filepath.Clean(absolute)] = text
	}
	governing := manifest
	root := filepath.Dir(filepath.Dir(manifest))
	for dir := filepath.Dir(root); ; dir = filepath.Dir(dir) {
		candidate := filepath.Join(dir, ".sec", "sec.toml")
		_, present := snapshots[candidate]
		if !present {
			info, err := os.Stat(candidate)
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			present = err == nil && !info.IsDir()
		}
		if present {
			governing = candidate
			root = dir
		}
		if filepath.Dir(dir) == dir {
			break
		}
	}
	// The selected manifest remains evidence even when reached through a
	// directory symlink, which the wider tree walk deliberately does not follow.
	found := map[string]bool{manifest: true}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if path == root && os.IsNotExist(err) {
				return nil // An entirely unsaved project can exist only in overlays.
			}
			return err
		}
		if !entry.IsDir() && entry.Name() == "sec.toml" && filepath.Base(filepath.Dir(path)) == ".sec" {
			found[filepath.Clean(path)] = true
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover project manifests: %w", err)
	}
	for path := range snapshots {
		if filepath.Base(path) != "sec.toml" || filepath.Base(filepath.Dir(path)) != ".sec" {
			continue
		}
		relative, err := filepath.Rel(root, path)
		if err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			found[path] = true
		}
	}
	paths := make([]string, 0, len(found))
	for path := range found {
		if path != governing {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	result := make([]BoundaryViolation, 0, len(paths))
	for _, path := range paths {
		text, present := snapshots[path]
		if !present {
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			text = string(data)
		}
		line := manifestKeyLine(text, []string{"project"})
		if line == 0 {
			line = 1
		}
		result = append(result, BoundaryViolation{Manifest: path, GoverningManifest: governing, Line: line})
	}
	return result, nil
}
