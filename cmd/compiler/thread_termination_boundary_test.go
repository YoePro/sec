package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPortableThreadTerminationBoundaryCLI shares the mandatory source
// diagnostic in human/JSON output and never publishes an invalid artifact.
// Rules: rules/concurrency/threads.md — §53(1–4);
// rules/tooling/diagnostics.md — §14.
func TestPortableThreadTerminationBoundaryCLI(t *testing.T) {
	valid := "../../testdata/sema/thread_termination_boundary_valid.sec"
	invalid := "../../testdata/sema/thread_termination_boundary_invalid.sec"
	_, output, code := runCLIForDiagnostics(t, "check", valid)
	if code != 0 {
		t.Fatal(code, output)
	}
	_, output, code = runCLIForDiagnostics(t, "check", invalid)
	if code != 3 || !strings.Contains(output, "S1227") || !strings.Contains(output, "RequestCancel()") || !strings.Contains(output, "unsafe target-specific API") {
		t.Fatal(code, output)
	}
	artifact := filepath.Join(t.TempDir(), "invalid.ir")
	_, output, code = runCLIForDiagnostics(t, "emit-ir", invalid, "--diagnostic-format=json", "-o", artifact)
	if code != 3 {
		t.Fatal(code, output)
	}
	doc := decodeOccurrenceDocument(t, output)
	if doc.Summary.Errors != 9 || len(doc.Occurrences) != 9 {
		t.Fatal(output)
	}
	for _, occurrence := range doc.Occurrences {
		if occurrence.ID == nil || *occurrence.ID != "S1227" || len(occurrence.Help) == 0 {
			t.Fatal(occurrence)
		}
	}
	if _, err := os.Stat(artifact); !os.IsNotExist(err) {
		t.Fatal("invalid termination emitted an artifact", err)
	}
}
