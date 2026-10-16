package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMapEntryIterationCLI accepts the frontend forms and stops invalid binding
// modes or structural mutation before emitting output. Runtime map lowering
// remains an explicit independent representation boundary.
// Rules: rules/collections/collections.md — §14.6;
// rules/control-flow/flowcontrol_for.md — §21.
func TestMapEntryIterationCLI(t *testing.T) {
	if _, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/map_iteration_valid.sec", "--diagnostic-format=json"); code != 0 {
		t.Fatal(code, stderr)
	}
	path := filepath.Join(t.TempDir(), "existing.ir")
	if err := os.WriteFile(path, []byte("existing output"), 0600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/map_iteration_invalid.sec", "-o", path, "--diagnostic-format=json")
	if code != 3 || stdout != "" || decodeOccurrenceDocument(t, stderr).Summary.Errors != 8 {
		t.Fatal(code, stdout, stderr)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "existing output" {
		t.Fatal(string(data), err)
	}
}
