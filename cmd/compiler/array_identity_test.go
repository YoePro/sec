package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestArrayOwnerIdentityCLI accepts valid fixed equality and owning transfers,
// and rejects representation/nominal equality confusion before output emission.
// Rules: rules/collections/collections.md — §§6.1,10–12;
// rules/types/types.md — "Named types".
func TestArrayOwnerIdentityCLI(t *testing.T) {
	if _, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/array_identity_valid.sec", "--diagnostic-format=json"); code != 0 {
		t.Fatal(code, stderr)
	}
	for _, test := range []struct {
		file   string
		errors int
	}{{"array_identity_invalid.sec", 5}, {"array_nominal_equality_invalid.sec", 3}} {
		path := filepath.Join(t.TempDir(), "existing.ir")
		if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/"+test.file, "-o", path, "--diagnostic-format=json")
		if code != 3 || stdout != "" || decodeOccurrenceDocument(t, stderr).Summary.Errors != test.errors {
			t.Fatal(test.file, code, stdout, stderr)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "existing output" {
			t.Fatal(string(data), err)
		}
	}
}
