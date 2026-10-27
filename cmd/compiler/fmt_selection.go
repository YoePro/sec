package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sec/internal/formatconfig"
)

// fmtSelection is one source file chosen by `sec fmt`: its normalized
// absolute path, the stable spelling shown in diagnostics, and the bytes and
// permissions read during preflight.
type fmtSelection struct {
	absolute string
	display  string
	input    []byte
	mode     os.FileMode
}

// selectFmtSources expands explicit file and directory arguments into the
// frozen, deduplicated and sorted set of files to format. Directories are
// walked recursively without following symbolic links, keeping exact `.sec`
// files and skipping hidden directories, `testdata/`, the project-reserved
// `bin/`, and stopping at a nested project boundary. Explicit files are kept
// as given. Every file is read here, before anything is written.
// Rules: rules/tooling/formatter.md — §§ 28(9)–28(13) (MD-037);
// rules/projects/projects.md — §§ 4(4), 6(12), 15.3.
func selectFmtSources(arguments []string) ([]fmtSelection, error) {
	byPath := map[string]fmtSelection{}
	add := func(path, display string) error {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("%s: %w", display, err)
		}
		if _, seen := byPath[absolute]; !seen {
			byPath[absolute] = fmtSelection{absolute: absolute, display: display}
		}
		return nil
	}
	for _, argument := range arguments {
		info, err := os.Lstat(argument)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", argument, err)
		}
		switch {
		case info.Mode()&fs.ModeSymlink != 0:
			return nil, fmt.Errorf("%s: symbolic links are not followed; name the target path instead", argument)
		case info.Mode().IsRegular():
			if err := add(argument, argument); err != nil {
				return nil, err
			}
		case info.IsDir():
			if err := walkFmtDirectory(argument, add); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("%s is neither a regular source file nor a directory", argument)
		}
	}
	selected := make([]fmtSelection, 0, len(byPath))
	for _, selection := range byPath {
		selected = append(selected, selection)
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].absolute < selected[j].absolute })
	for index := range selected {
		info, err := os.Lstat(selected[index].absolute)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", selected[index].display, err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%s is not a regular source file", selected[index].display)
		}
		input, err := os.ReadFile(selected[index].absolute)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", selected[index].display, err)
		}
		selected[index].input, selected[index].mode = input, info.Mode().Perm()
	}
	return selected, nil
}

// walkFmtDirectory adds the eligible `.sec` files below root.
// Rules: rules/tooling/formatter.md — §§ 28(10)–28(11).
func walkFmtDirectory(root string, add func(path, display string) error) error {
	// `bin/` is reserved only directly below the project root, or below the
	// scanned directory itself when it belongs to no project.
	reservedParent, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("%s: %w", root, err)
	}
	if manifest, found := formatconfig.ProjectManifest(root); found {
		reservedParent = filepath.Dir(filepath.Dir(manifest))
	}
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if !entry.IsDir() {
			if entry.Type().IsRegular() && strings.HasSuffix(entry.Name(), ".sec") {
				return add(path, path)
			}
			return nil
		}
		if path == root {
			return nil
		}
		name := entry.Name()
		absolute, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if strings.HasPrefix(name, ".") || name == "testdata" || name == "bin" && filepath.Dir(absolute) == reservedParent {
			return filepath.SkipDir
		}
		if info, err := os.Stat(filepath.Join(path, ".sec", "sec.toml")); err == nil && !info.IsDir() {
			return fmt.Errorf("%s: project boundary: a separate Sec project manifest lies inside the selected directory %s; format it separately", path, root)
		}
		return nil
	})
}
