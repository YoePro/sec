package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/project"
)

// TestInitIdentityConformance exercises the CLI across rename and repeated init,
// and proves invalid identities abort without changing the manifest or layout.
// Rules: rules/projects/projects.md — §3.3,§7,§37(4),§40(3),§50(1).
func TestInitIdentityConformance(t *testing.T) {
	dir := t.TempDir()
	_, stderr, code := runCLIForDiagnostics(t, "init", dir, "--minimal", "--name", "Original")
	if code != 0 {
		t.Fatal(code, stderr)
	}
	manifest := filepath.Join(dir, ".sec", "sec.toml")
	original, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	identity, errors := project.ParseManifestIdentity(string(original))
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	if identity.UUID[6]>>4 != 4 || identity.UUID[8]&0xc0 != 0x80 {
		t.Fatal(identity.UUID)
	}
	renamed := strings.Replace(string(original), `name = "Original"`, `name = "Renamed"`, 1)
	if err := os.WriteFile(manifest, []byte(renamed), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Original", "Renamed"} {
		_, stderr, code = runCLIForDiagnostics(t, "init", dir, "--name", name)
		if code != 0 {
			t.Fatal(code, stderr)
		}
		got, err := os.ReadFile(manifest)
		if err != nil || string(got) != renamed {
			t.Fatal("init changed renamed identity", err)
		}
		next, errors := project.ParseManifestIdentity(string(got))
		if len(errors) != 0 || next.UUID != identity.UUID || next.Name != "Renamed" {
			t.Fatal(next, errors)
		}
	}
	for _, invalid := range []string{"00000000-0000-0000-0000-000000000000", "broken-uuid"} {
		bad := strings.Replace(renamed, identity.UUID.String(), invalid, 1)
		if err := os.WriteFile(manifest, []byte(bad), 0644); err != nil {
			t.Fatal(err)
		}
		_, stderr, code = runCLIForDiagnostics(t, "init", dir, "--name", "Renamed")
		if code == 0 || !strings.Contains(stderr, "project.uuid") {
			t.Fatal(code, stderr)
		}
		got, err := os.ReadFile(manifest)
		if err != nil || string(got) != bad {
			t.Fatal("failed init changed manifest", err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 2 {
			t.Fatal(entries, err)
		}
	}
}
