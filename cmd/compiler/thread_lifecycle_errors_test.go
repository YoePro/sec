package main

import (
	"strings"
	"testing"
)

// TestThreadLifecycleErrorsCLI checks exact portable error types through the
// compiler entry point, including negative diagnostics for legacy variants.
// Rules: rules/concurrency/threads.md — §§18–19.
func TestThreadLifecycleErrorsCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_lifecycle_errors_valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_lifecycle_errors_invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, output)
	}
	doc := decodeOccurrenceDocument(t, output)
	if doc.Summary.Errors != 3 {
		t.Fatal(output)
	}
	for _, name := range []string{"ThreadSpawnError.Unsupported", "ThreadSpawnError.ResourceUnavailable", "ThreadStartError.Unsupported"} {
		if !strings.Contains(output, name+" has never been declared") {
			t.Fatal(output)
		}
	}
}
