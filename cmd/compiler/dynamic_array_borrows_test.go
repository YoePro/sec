package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDynamicArrayMutationBorrowsCLI diagnoses invalidating mutations before
// lowering and leaves an existing output untouched.
// Rules: rules/collections/collections.md — §§6.5,19;
// rules/memory/borrowing.md — §14.5.
func TestDynamicArrayMutationBorrowsCLI(t *testing.T) {
	if _, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/dynamic_array_borrows_valid.sec", "--diagnostic-format=json"); code != 0 {
		t.Fatal(code, stderr)
	}
	path := filepath.Join(t.TempDir(), "output.ir")
	if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/dynamic_array_borrows_invalid.sec", "-o", path, "--diagnostic-format=json")
	if code != 3 || stdout != "" || decodeOccurrenceDocument(t, stderr).Summary.Errors != 10 {
		t.Fatal(code, stdout, stderr)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "existing output" {
		t.Fatal(string(data), err)
	}
}
