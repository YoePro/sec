package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The public CLI check/write/check cycle exposes canonical consuming syntax
// without enabling language corrections or inferring additional transfers.
// Rules: rules/memory/copy_move.md §29(3); rules/tooling/formatter.md §§21,28,29.
func TestFmtOwnershipMarkersCLI(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/ownership_markers.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/ownership_markers.expected.sec")
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
