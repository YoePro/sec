package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SourceScope is the source boundary selected by an enclosing manifest, or a
// standalone directory when no manifest exists. It is an immutable request fact;
// callers rediscover it after manifest edits instead of retaining a stale root.
// Rules: rules/projects/projects.md — §§4,13–15,36(1–3),49(1–3).
type SourceScope struct {
	Root     string
	Manifest string
}

// DiscoverSourceScope searches a directory and its parents for the nearest
// manifest. An unreadable or invalid manifest is an error, never permission to
// treat its project as an unrestricted standalone folder.
// Rules: rules/projects/projects.md — §§5(1),36(1–3),48(2),49(1–3).
func DiscoverSourceScope(directory string) (SourceScope, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return SourceScope{}, err
	}
	absolute = filepath.Clean(absolute)
	for current := absolute; ; current = filepath.Dir(current) {
		path := filepath.Join(current, ".sec", "sec.toml")
		_, err := os.Stat(path)
		if err == nil {
			manifest, err := LoadManifest(path)
			if err != nil {
				return SourceScope{}, err
			}
			// Discovery must not turn a forbidden nested manifest into a new
			// import/source root, including below a loose umbrella folder.
			for parent := filepath.Dir(current); parent != current; parent = filepath.Dir(parent) {
				outer := filepath.Join(parent, ".sec", "sec.toml")
				if _, err := os.Stat(outer); err == nil {
					return SourceScope{}, fmt.Errorf("nested Project manifest %s inside governing Project %s", path, outer)
				} else if !os.IsNotExist(err) {
					return SourceScope{}, err
				}
				if filepath.Dir(parent) == parent {
					break
				}
			}
			return SourceScope{Root: manifest.Facts().ProjectRoot, Manifest: manifest.Path()}, nil
		}
		if !os.IsNotExist(err) {
			return SourceScope{}, err
		}
		if filepath.Dir(current) == current {
			return SourceScope{Root: absolute}, nil
		}
	}
}

// Contains checks lexical containment and the canonical project source layout.
// Descendant manifest boundaries are resolved separately by the caller using
// DiscoverSourceScope; this operation does not traverse or read source files.
// Rules: rules/projects/projects.md — §§13–15,36; rules/projects/modules.md — Source directory and module membership.
func (scope SourceScope) Contains(source string) bool {
	if scope.Root == "" || filepath.Ext(source) != ".sec" {
		return false
	}
	relative, err := filepath.Rel(scope.Root, source)
	if err != nil || filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	if scope.Manifest != "" {
		_, err := SourceLocation(scope.Root, source)
		return err == nil
	}
	return true
}
