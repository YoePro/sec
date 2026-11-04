package project

import (
	"os"
	"path/filepath"
	"testing"
)

// Shared project source facts follow enclosing manifests, source layout and
// request-time edits without changing a standalone directory into a project.
// Rules: rules/projects/projects.md — §§4,13–15,36,49.
func TestDiscoverSourceScope(t *testing.T) {
	root := t.TempDir()
	directory := filepath.Join(root, "services", "store")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	loose, err := DiscoverSourceScope(directory)
	if err != nil || loose.Root != directory || loose.Manifest != "" {
		t.Fatal(loose, err)
	}
	manifest := filepath.Join(root, ".sec", "sec.toml")
	if err := os.MkdirAll(filepath.Dir(manifest), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(manifestFixture(t)), 0600); err != nil {
		t.Fatal(err)
	}
	scope, err := DiscoverSourceScope(directory)
	if err != nil || scope.Root != root || scope.Manifest != manifest {
		t.Fatal(scope, err)
	}
	for _, name := range []string{"main.sec", "cmd/app/main.sec", "internal/store/store.sec", "services/bin/store.sec"} {
		if !scope.Contains(filepath.Join(root, name)) {
			t.Fatal("missing project source", name)
		}
	}
	for _, name := range []string{"../other.sec", "bin/output.sec", ".sec/cache.sec", "cmd/main.sec", "services/internal/direct.sec", "notes.txt"} {
		if scope.Contains(filepath.Join(root, name)) {
			t.Fatal("accepted non-source", name)
		}
	}
	nested := filepath.Join(directory, ".sec", "sec.toml")
	if err := os.MkdirAll(filepath.Dir(nested), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte(manifestFixture(t)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := DiscoverSourceScope(directory); err == nil {
		t.Fatal("nested manifest became an independent source root")
	}
	if err := os.Remove(nested); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte("[project]\nname = 12\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := DiscoverSourceScope(directory); err == nil {
		t.Fatal("invalid manifest enabled loose fallback")
	}
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	after, err := DiscoverSourceScope(directory)
	if err != nil || after != loose {
		t.Fatal("stale project root", after, err)
	}
}
