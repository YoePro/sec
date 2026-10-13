package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Rules: rules/projects/projects.md — §14; rules/tooling/diagnostics.md — §§5,9.
func TestCLIProjectRootModule(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(fixture), `module = "suite"`, `module = "codec"`, 1)
	manifest := filepath.Join(root, ".sec", "sec.toml")
	if err := os.WriteFile(manifest, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/projects/root_module/library.sec")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "library.sec")
	if err := os.WriteFile(path, source, 0644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := runCLIForDiagnostics(t, "sema", path)
	if code != 0 {
		t.Fatal(code, stderr)
	}
	// Renaming the display name does not change a root override.
	renamed := strings.Replace(text, "SEC Compiler Suite", "Renamed human project", 1)
	if err := os.WriteFile(manifest, []byte(renamed), 0644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = runCLIForDiagnostics(t, "sema", path)
	if code != 0 {
		t.Fatal(code, stderr)
	}
	wrong, err := os.ReadFile("../../testdata/projects/root_module/mismatched_module.sec")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, wrong, 0644); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "sema", "--diagnostic-format=json", path)
	if code == 0 || stdout != "" || !strings.Contains(stderr, "S1148") || !strings.Contains(stderr, "expected") {
		t.Fatal(code, stdout, stderr)
	}
	// Even a sibling excluded from same-name assembly must report its mismatch.
	if err := os.WriteFile(path, source, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "other.sec"), wrong, 0644); err != nil {
		t.Fatal(err)
	}
	_, stderr, code = runCLIForDiagnostics(t, "sema", path)
	if code == 0 || !strings.Contains(stderr, "S1148") || !strings.Contains(stderr, "other.sec") {
		t.Fatal(code, stderr)
	}
}
