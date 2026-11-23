package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProductionTestTargetCLI rejects test-only production entries at manifest
// validation, before analysis/emission/linking or build-counter mutation.
// Rules: rules/tooling/testing.md — §§8.1,26.5;
// rules/projects/projects.md — §§17–18,48.
func TestProductionTestTargetCLI(t *testing.T) {
	root := t.TempDir()
	fixture := "../../testdata/projects/production_test_target"
	for _, name := range []string{".sec/sec.toml", "tests/product/product.sec"} {
		stored := name
		if name == ".sec/sec.toml" {
			stored = "manifest.toml" // Activate the manifest only in the isolated project.
		}
		data, err := os.ReadFile(filepath.Join(fixture, stored))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := filepath.Join(root, ".sec", "sec.toml")
	original, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(root, "tests", "product", "product.sec")
	artifact := filepath.Join(root, "artifact")
	for _, args := range [][]string{
		{"check", source}, {"sema", source}, {"analyse", source}, {"emit-ir", source, "-o", artifact},
		{"build", source, "-o", artifact}, {"test", "--list", root},
	} {
		args = append(args, "--diagnostic-format=json")
		_, output, code := runCLIForDiagnostics(t, args...)
		doc := decodeOccurrenceDocument(t, output)
		if code != 3 || len(doc.Occurrences) != 1 {
			t.Fatal(args, code, output)
		}
		e := doc.Occurrences[0]
		if e.ID == nil || *e.ID != "S1145" || e.Primary == nil || e.Primary.Span.File != manifest || e.Primary.Span.Start.Line != 14 {
			t.Fatal(e)
		}
	}
	output, err := runBuildCLIInDirectory(t, root, "product", "--diagnostic-format=json")
	if err == nil || !strings.Contains(output, "S1145") {
		t.Fatal("named target bypass", output, err)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("artifact published", err)
	}
	current, err := os.ReadFile(manifest)
	if err != nil || string(current) != string(original) {
		t.Fatal("manifest changed", err)
	}
}
