package project

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// Rules: rules/projects/projects.md — §§4,36,48.
func TestProjectBoundaryDiskAndOverlayDiscovery(t *testing.T) {
	root := t.TempDir()
	text, err := os.ReadFile("../../testdata/projects/boundary/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	write := func(path string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, text, 0600); err != nil {
			t.Fatal(err)
		}
	}
	outer := filepath.Join(root, ".sec", "sec.toml")
	child := filepath.Join(root, "child", ".sec", "sec.toml")
	grandchild := filepath.Join(root, "child", "deep", ".sec", "sec.toml")
	write(outer)
	if got, err := ValidateBoundary(outer, nil); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	write(child)
	write(grandchild)
	before, _ := os.ReadFile(child)
	for _, selected := range []string{outer, child, grandchild} {
		got, err := ValidateBoundary(selected, nil)
		if err != nil || len(got) != 2 || got[0].Manifest != child || got[1].Manifest != grandchild || got[0].GoverningManifest != outer || got[0].Line != 2 {
			t.Fatal(selected, got, err)
		}
	}
	after, _ := os.ReadFile(child)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("discovery changed manifest contents")
	}
	if err := os.Remove(child); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(grandchild); err != nil {
		t.Fatal(err)
	}
	unsaved := filepath.Join(root, "unsaved", ".sec", "sec.toml")
	out := filepath.Join(root+"-other", ".sec", "sec.toml")
	got, err := ValidateBoundary(outer, map[string]string{unsaved: string(text), out: string(text)})
	if err != nil || len(got) != 1 || got[0].Manifest != unsaved {
		t.Fatal(got, err)
	}
	got, err = ValidateBoundary(unsaved, map[string]string{unsaved: string(text), outer: string(text)})
	if err != nil || len(got) != 1 || got[0].GoverningManifest != outer {
		t.Fatal(got, err)
	}
	if got, err := ValidateBoundary(outer, nil); err != nil || len(got) != 0 {
		t.Fatal("removed overlays retained", got, err)
	}
}

func TestProjectBoundaryDoesNotFollowDirectorySymlinks(t *testing.T) {
	root, external := t.TempDir(), t.TempDir()
	text, err := os.ReadFile("../../testdata/projects/boundary/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{root, external} {
		if err := os.Mkdir(filepath.Join(dir, ".sec"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, ".sec", "sec.toml"), text, 0600); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(root, "linked")
	if err := os.Symlink(external, link); err != nil {
		t.Skip(err)
	}
	outer := filepath.Join(root, ".sec", "sec.toml")
	if got, err := ValidateBoundary(outer, nil); err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	if got, err := ValidateBoundary(filepath.Join(link, ".sec", "sec.toml"), nil); err != nil || len(got) != 1 {
		t.Fatal("selected nested manifest bypassed through symlink", got, err)
	}
}
