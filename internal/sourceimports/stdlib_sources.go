package sourceimports

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"sec/internal/modules"
)

// StandardLibrarySources discovers a canonical library module before project
// fallback. Directory modules include all production siblings, not just a
// same-named entry file. Test-only sources do not become production imports.
// Rules: rules/projects/modules.md — §§3,8,9;
// rules/tooling/testing.md — §25 test-only dependency direction.
func StandardLibrarySources(root, path string) ([]string, bool) {
	path = strings.TrimPrefix(path, "std/")
	if modules.ValidateImportPath(path) != nil || strings.HasPrefix(path, "platform/") {
		return nil, false
	}
	base := filepath.Join(root, "sec", "stdlib", filepath.FromSlash(path))
	if info, err := os.Stat(base + ".sec"); err == nil && !info.IsDir() {
		return []string{filepath.Clean(base + ".sec")}, true
	}
	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		return nil, false
	}
	matches, err := filepath.Glob(filepath.Join(base, "*.sec"))
	if err != nil {
		return nil, false
	}
	paths := []string{}
	for _, path := range matches {
		if !strings.HasSuffix(path, "_test.sec") {
			paths = append(paths, filepath.Clean(path))
		}
	}
	sort.Strings(paths)
	return paths, len(paths) != 0
}
