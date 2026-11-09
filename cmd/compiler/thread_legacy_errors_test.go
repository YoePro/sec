package main

import (
	"strings"
	"testing"
)

// TestThreadLegacyErrorsCLI permits canonical attachment errors and ordinary
// project-owned names, but exposes no implicit portable legacy thread errors.
// Rules: rules/concurrency/threads.md — §3;
// rules/concurrency/thread_local.md — §§29,37.
func TestThreadLegacyErrorsCLI(t *testing.T) {
	for _, name := range []string{"valid", "local"} {
		_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_legacy_errors_"+name+".sec")
		if code != 0 {
			t.Fatal(code, output)
		}
	}
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/thread_legacy_errors_invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, output)
	}
	doc := decodeOccurrenceDocument(t, output)
	if doc.Summary.Errors != 3 {
		t.Fatal(output)
	}
	for _, name := range []string{"ThreadSchedulingError", "ThreadTerminationError", "ThreadContextError"} {
		if !strings.Contains(output, "unknown type "+name) {
			t.Fatal(output)
		}
	}
}
