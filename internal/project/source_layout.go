package project

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SourceLocation returns the project-relative source directory, rejecting
// metadata/output storage and direct files in organizational/access markers.
// A nested directory named bin or cmd is ordinary: reservation is root-only.
// Rules: rules/projects/projects.md — §§14,15.1–15.5.
func SourceLocation(root, source string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	source, err = filepath.Abs(source)
	if err != nil {
		return "", err
	}
	relative, err := filepath.Rel(root, filepath.Dir(source))
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", nil
	}
	if relative == "." {
		return relative, nil
	}
	path := filepath.ToSlash(relative)
	parts := strings.Split(path, "/")
	if parts[0] == "bin" {
		return path, fmt.Errorf("project source is forbidden below bin; bin is generated output storage")
	}
	if parts[0] == ".sec" {
		return path, fmt.Errorf("source modules are forbidden below .sec; .sec is project metadata storage")
	}
	if path == "cmd" {
		return path, fmt.Errorf("source files are forbidden directly in cmd; place an entry module in a child directory")
	}
	if parts[len(parts)-1] == "internal" {
		return path, fmt.Errorf("source files are forbidden directly in an internal access marker; place the module in a child directory")
	}
	return path, nil
}

// ProjectImportAccess checks reserved root storage and every internal marker
// before imported declarations can make an inaccessible module appear public.
// Rules: rules/projects/projects.md — §15.3(3),§15.4(2),§15.5(5–7);
// rules/projects/modules.md — §13 "internal module access".
func ProjectImportAccess(root, source, path string) error {
	directory, err := SourceLocation(root, source)
	if err != nil {
		return nil
	} // the source declaration owns its location failure
	if directory == "" {
		return nil
	} // source belongs to another import root
	parts := strings.Split(path, "/")
	if parts[0] == "bin" || parts[0] == ".sec" {
		return fmt.Errorf("cannot import %q: %s is reserved project storage", path, parts[0])
	}
	for index, part := range parts {
		if part != "internal" {
			continue
		}
		if index == len(parts)-1 {
			return fmt.Errorf("cannot import %q: internal is an access marker, not a source module", path)
		}
		parent := strings.Join(parts[:index], "/")
		if parent != "" && directory != parent && !strings.HasPrefix(directory, parent+"/") {
			return fmt.Errorf("cannot import %q from %q: internal modules are accessible only below %q", path, directory, parent)
		}
	}
	return nil
}
