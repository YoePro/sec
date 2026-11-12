package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCTETransientStorageCLI shares deterministic resource diagnostics with
// Sema and never writes an IR artifact after failed required evaluation.
// Rules: rules/compiler/compile_time_evaluation.md — §§24–25,48–49,59.
func TestCTETransientStorageCLI(t *testing.T) {
	for _, file := range []string{"cte_transient_storage_valid.sec", "cte_transient_strings_valid.sec"} {
		_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/"+file)
		if code != 0 {
			t.Fatal(code, output)
		}
	}
	invalid := "../../testdata/sema/cte_transient_storage_invalid.sec"
	_, output, code := runCLIForDiagnostics(t, "check", invalid)
	if code != 3 || !strings.Contains(output, "S1225") || !strings.Contains(output, "S1224") {
		t.Fatal(code, output)
	}
	artifact := filepath.Join(t.TempDir(), "invalid.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", invalid, "--diagnostic-format=json", "-o", artifact)
	if code != 3 {
		t.Fatal(code, output)
	}
	doc := decodeOccurrenceDocument(t, output)
	if doc.Summary.Errors != 3 || !strings.Contains(output, "AggregateValueSize") || !strings.Contains(output, "TransientMemory") || strings.Contains(output, "CompilerInvariantFailure") {
		t.Fatal(output)
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("invalid CTE wrote an artifact", err)
	}
}
