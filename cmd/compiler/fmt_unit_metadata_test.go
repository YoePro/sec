package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The public CLI rejects noncanonical metadata spacing, writes the shared
// golden, and accepts a second check without rewriting legacy enum values.
// Rules: rules/types/units.md — "Formatter requirements";
// rules/tooling/formatter.md §§22(11–13),28,29.
func TestFmtUnitMetadataSpacingCLI(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/unit_metadata_spacing.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/unit_metadata_spacing.expected.sec")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	file := filepath.Join(root, "main.sec")
	if err := os.WriteFile(file, input, 0600); err != nil {
		t.Fatal(err)
	}
	if code, report := runBulkFmtCLI(t, root, "--check", file); code != 1 {
		t.Fatalf("initial check: %d %s", code, report)
	}
	if got := readFile(t, file); got != string(input) {
		t.Fatal("check changed source")
	}
	if code, report := runBulkFmtCLI(t, root, file); code != 0 {
		t.Fatalf("format: %d %s", code, report)
	}
	if got := readFile(t, file); got != string(want) {
		t.Fatalf("CLI differs from golden:\n%s", got)
	}
	if code, report := runBulkFmtCLI(t, root, "--check", file); code != 0 || report != "" {
		t.Fatalf("second check: %d %s", code, report)
	}
}
