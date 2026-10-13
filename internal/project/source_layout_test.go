package project

import (
	"path/filepath"
	"testing"
)

// Rules: rules/projects/projects.md — §§14–15.
func TestProjectSourceLocations(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"main.sec", "cmd/worker/main.sec", "services/internal/store/store.sec", "internal/store/store.sec", "services/bin/store.sec", "services/cmd/store.sec"} {
		if location, err := SourceLocation(root, filepath.Join(root, path)); err != nil || location == "" {
			t.Fatal(path, location, err)
		}
	}
	for _, path := range []string{"cmd/main.sec", "bin/store.sec", "bin/a/b/store.sec", ".sec/store.sec", ".sec/a/store.sec", "internal/store.sec", "services/internal/store.sec"} {
		if _, err := SourceLocation(root, filepath.Join(root, path)); err == nil {
			t.Fatal("accepted reserved source", path)
		}
	}
	if location, err := SourceLocation(root, filepath.Join(root, "..", "other", "bin", "source.sec")); err != nil || location != "" {
		t.Fatal("another root reinterpreted", location, err)
	}
}

// Rules: rules/projects/projects.md — §15.3–15.5; rules/projects/modules.md — §13.
func TestProjectInternalImportAccess(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct {
		source, path string
		allowed      bool
	}{
		{"main.sec", "internal/store", true},
		{"services/client/client.sec", "services/internal/store", true},
		{"services_extra/client/client.sec", "services/internal/store", false},
		{"client/client.sec", "services/internal/store", false},
		{"services/internal/deeper/client/client.sec", "services/internal/deeper/internal/store", true},
		{"services/client/client.sec", "services/internal/deeper/internal/store", false},
		{"client/client.sec", "internal", false},
		{"client/client.sec", "bin/store", false},
		{"client/client.sec", ".sec/store", false},
		{"client/client.sec", "services/bin/store", true},
	} {
		if err := ProjectImportAccess(root, filepath.Join(root, tc.source), tc.path); (err == nil) != tc.allowed {
			t.Fatal(tc, err)
		}
	}
}
