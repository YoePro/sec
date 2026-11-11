package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCancellationContextCLI accepts the canonical source surface and rejects
// malformed calls, temporal types, authority, representation and copy attempts
// before publishing any IR artifact.
// Rules: rules/concurrency/cancellation.md — §§33,35–38,43.
func TestCancellationContextCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/context_surface_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	artifact := filepath.Join(t.TempDir(), "invalid.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/context_surface_invalid.sec", "--diagnostic-format=json", "-o", artifact)
	if code != 3 || decodeOccurrenceDocument(t, output).Summary.Errors != 14 {
		t.Fatal(code, output)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("invalid Context surface emitted IR", err)
	}
}
