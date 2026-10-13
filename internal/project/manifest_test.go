package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func manifestFixture(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// Rules: rules/projects/projects.md — §§3.2–3.3,7,8,11,49(1–3).
func TestManifestResolvedFacts(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".sec", "sec.toml")
	manifest, diagnostics := ParseManifest(path, manifestFixture(t))
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	facts := manifest.Facts()
	if facts.ProjectRoot != root || facts.ProjectName != "SEC Compiler Suite" || facts.ProjectUUID.String() != "550e8400-e29b-41d4-a716-446655440000" {
		t.Fatal(facts)
	}
	rendered, err := facts.ProjectVersion.Render()
	if err != nil || rendered != "1.0.3-alpha.123" {
		t.Fatal(rendered, err)
	}
	for name, want := range map[string]string{"sec": "1.0.3-alpha.421", "lsp": "0.8.2-beta.97", "library": "1.0.3-alpha.123"} {
		version, found := manifest.TargetVersion(name)
		if !found {
			t.Fatal(name)
		}
		got, err := version.Render()
		if err != nil || got != want {
			t.Fatal(name, got, err)
		}
	}
	if _, found := manifest.TargetVersion("unknown"); found {
		t.Fatal("invented a Target version")
	}
	if manifest.Identity().Module != "suite" || manifest.Text() != manifestFixture(t) || manifest.Path() != path {
		t.Fatal(manifest)
	}
	// Returned values cannot mutate the snapshot or leak shared mutable slices.
	facts.ProjectName = "changed"
	facts.ProjectVersion.Build = 999
	targets := manifest.Targets()
	targets[0].Name = "changed"
	if manifest.Facts().ProjectName != "SEC Compiler Suite" || manifest.Facts().ProjectVersion.Build != 123 || manifest.Targets()[0].Name == "changed" {
		t.Fatal("mutable fact aliases")
	}
}

// Rules: rules/projects/projects.md — §§3.3(6–8),37(4),49(1–3).
func TestManifestSnapshotMoveRenameAndReload(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "original")
	path := filepath.Join(root, ".sec", "sec.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	original := manifestFixture(t)
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	first, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	renamed := strings.Replace(original, "SEC Compiler Suite", "A different display name", 1)
	renamed = strings.Replace(renamed, "build = 123", "build = 124", 1)
	if err := os.WriteFile(path, []byte(renamed), 0644); err != nil {
		t.Fatal(err)
	}
	second, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if first.Facts().ProjectName != "SEC Compiler Suite" || first.Facts().ProjectVersion.Build != 123 || second.Facts().ProjectVersion.Build != 124 || second.Facts().ProjectName != "A different display name" || first.Facts().ProjectUUID != second.Facts().ProjectUUID {
		t.Fatal(first.Facts(), second.Facts())
	}
	moved := filepath.Join(parent, "moved")
	if err := os.Rename(root, moved); err != nil {
		t.Fatal(err)
	}
	third, err := LoadManifest(filepath.Join(moved, ".sec", "sec.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if third.Facts().ProjectRoot != moved || third.Facts().ProjectUUID != first.Facts().ProjectUUID || third.Facts().ProjectVersion != second.Facts().ProjectVersion || first.Facts().ProjectRoot != root {
		t.Fatal(third.Facts())
	}
	// Accessors retain all facts after the source is deleted; they perform no I/O.
	if err := os.Remove(filepath.Join(moved, ".sec", "sec.toml")); err != nil {
		t.Fatal(err)
	}
	if third.Facts().ProjectName != "A different display name" {
		t.Fatal(third.Facts())
	}
	if version, found := third.TargetVersion("sec"); !found || version.Build != 421 {
		t.Fatal(version, found)
	}
}

// Rules: rules/projects/projects.md — §§7,8,48(2),49(1–2).
func TestManifestRejectsInvalidFacts(t *testing.T) {
	text := manifestFixture(t)
	for _, bad := range []string{
		strings.Replace(text, "550e8400-e29b-41d4-a716-446655440000", "00000000-0000-0000-0000-000000000000", 1),
		strings.Replace(text, "major = 1", "major = -1", 1),
		strings.Replace(text, "build = 123", "build = 0x123", 1),
	} {
		manifest, diagnostics := ParseManifest(filepath.Join(t.TempDir(), ".sec", "sec.toml"), bad)
		if manifest != nil || len(diagnostics) == 0 {
			t.Fatal("published invalid project facts", manifest, diagnostics)
		}
	}
	if manifest, diagnostics := ParseManifest("sec.toml", text); manifest != nil || len(diagnostics) != 1 {
		t.Fatal(manifest, diagnostics)
	}
	path := filepath.Join(t.TempDir(), ".sec", "sec.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("[project]\nname = ''\n"), 0644); err != nil {
		t.Fatal(err)
	}
	manifest, err := LoadManifest(path)
	var failure *ManifestErrors
	if manifest != nil || !errors.As(err, &failure) || len(failure.Diagnostics) < 2 || !strings.Contains(err.Error(), path) {
		t.Fatal(manifest, err)
	}
	if _, err := LoadManifest(filepath.Join(t.TempDir(), "missing")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// Rules: rules/projects/projects.md — §7(5),§49(1–2).
func TestManifestConcurrentReadersAndWarningIsolation(t *testing.T) {
	text := strings.Replace(manifestFixture(t), "SEC Compiler Suite", " SEC Compiler Suite ", 1)
	manifest, diagnostics := ParseManifest(filepath.Join(t.TempDir(), ".sec", "sec.toml"), text)
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	identity := manifest.Identity()
	if len(identity.Warnings) != 1 {
		t.Fatal(identity)
	}
	identity.Warnings[0].Message = "changed"
	if manifest.Identity().Warnings[0].Message == "changed" {
		t.Fatal("warning aliases snapshot")
	}
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 100; j++ {
				if manifest.Facts().ProjectName != " SEC Compiler Suite " {
					t.Error("name changed")
				}
				version, found := manifest.TargetVersion("sec")
				if !found || version.Build != 421 {
					t.Error("Target version changed")
				}
				_ = manifest.Identity()
				_ = manifest.Targets()
			}
		}()
	}
	group.Wait()
}

// Rules: rules/projects/projects.md — §§5(1),49(1–2).
func TestManifestRelativeRootAndDeterministicDiagnostics(t *testing.T) {
	path := filepath.Join("..", "project", ".sec", "sec.toml")
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	manifest, diagnostics := ParseManifest(path, manifestFixture(t))
	if len(diagnostics) != 0 || manifest.Facts().ProjectRoot != filepath.Dir(filepath.Dir(absolute)) || manifest.Path() != absolute {
		t.Fatal(manifest, diagnostics)
	}
	// Two independently invalid Target versions must not inherit map iteration order.
	invalid := strings.Replace(manifestFixture(t), "[target.sec.version]\nbuild = 421", "[target.sec.version]\nbuild = 421\nuserdefined = ''", 1)
	invalid = strings.Replace(invalid, "userdefined = \"beta\"", "userdefined = ''", 1)
	var previous string
	for i := 0; i < 20; i++ {
		result, diagnostics := ParseManifest(path, invalid)
		if result != nil || len(diagnostics) != 2 {
			t.Fatal(result, diagnostics)
		}
		current := diagnostics[0].Message + "\n" + diagnostics[1].Message
		if i > 0 && previous != current {
			t.Fatal("non-deterministic manifest diagnostics", previous, current)
		}
		previous = current
	}
}
