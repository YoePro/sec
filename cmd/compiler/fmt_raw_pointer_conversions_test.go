package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The CLI check/write/check cycle preserves RawPtr operation spelling and
// unsafe boundaries without inventing C-style pointer syntax.
// Rules: rules/memory/raw_pointers.md §§17(7–9),18A,36(5); rules/tooling/formatter.md §§17,19(11–14),28,29.
func TestFmtRawPointerConversionsCLI(t *testing.T) {
	input, err := os.ReadFile("../../testdata/formatter/raw_pointer_conversions.sec")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../testdata/formatter/raw_pointer_conversions.expected.sec")
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
