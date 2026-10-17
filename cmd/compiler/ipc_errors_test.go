package main

import (
	"strings"
	"testing"
)

// TestIPCErrorCLI loads the real core declarations and checks the exact closed
// errors and ordinary error-channel compatibility without requiring IPC runtime.
// Rules: rules/concurrency/ipc.md — §§6,27,44,55,68,79.
func TestIPCErrorCLI(t *testing.T) {
	for _, fixture := range []struct {
		name         string
		code, errors int
	}{
		{"errors_valid.sec", 0, 0}, {"error_values_valid.sec", 0, 0}, {"errors_invalid.sec", 3, 8}, {"errors_spoof_invalid.sec", 3, 5},
	} {
		_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/ipc/"+fixture.name, "--diagnostic-format=json")
		if code != fixture.code {
			t.Fatal(fixture.name, code, stderr)
		}
		if fixture.errors != 0 && decodeOccurrenceDocument(t, stderr).Summary.Errors != fixture.errors {
			t.Fatal(stderr)
		}
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/ipc/error_values_valid.sec")
	if code != 0 {
		t.Fatal(code, stderr)
	}
	for _, name := range []string{"IPCError", "SharedMemoryError", "IPCSyncError", "IPCSharedError", "IPCAtomicError"} {
		if !strings.Contains(stdout, "core::"+name) {
			t.Fatal(name, stdout)
		}
	}
	if !strings.Contains(stdout, "enum.constant") {
		t.Fatal(stdout)
	}
}
