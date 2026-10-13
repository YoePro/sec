package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Rules: rules/projects/projects.md — §§8–11,36,48(1–3); diagnostics.md — emitted occurrences.
func TestCLIDiscoveredProjectVersionDiagnostics(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "cmd", "sec"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".sec"), 0755); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("../../testdata/projects/model/sec.toml")
	if err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Join(root, ".sec", "sec.toml")
	bad := strings.Replace(string(fixture), "build = 123", "  build = -1 # invalid", 1)
	if err := os.WriteFile(manifest, []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile("../../testdata/projects/init/main.sec")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "cmd", "sec", "main.sec")
	if err := os.WriteFile(path, source, 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"sema", path}, {"check", path}, {"analyse", path}, {"emit-llvm", path, "-o", filepath.Join(root, "a.ll")},
		{"build", path, "-o", filepath.Join(root, "binary")},
		{"init", root, "--name", "Existing"},
		{"test", "--list", root},
	} {
		args = append(args, "--diagnostic-format=json")
		_, output, code := runCLIForDiagnostics(t, args...)
		if code == 0 {
			t.Fatal(args, output)
		}
		document := decodeOccurrenceDocument(t, output)
		if len(document.Occurrences) != 1 {
			t.Fatal(args, document)
		}
		item := document.Occurrences[0]
		if item.ID == nil || *item.ID != "S1150" || item.Primary == nil || item.Primary.Span.File != manifest || item.Primary.Span.Start.Column != 3 {
			t.Fatal(item)
		}
	}
	output, err := runBuildCLIInDirectory(t, root, "sec", "--diagnostic-format=json")
	if err == nil || !strings.Contains(output, "S1150") {
		t.Fatal("named Target bypassed version validation", output, err)
	}
}
