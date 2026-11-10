package main

import "testing"

// TestCommandConfigurationErrorCLI keeps the IPC configuration-error addition
// available to ordinary source and verified IR without inventing runtime I/O.
// Rules: rules/concurrency/ipc.md — §97; rules/concurrency/processes.md — §5.9.
func TestCommandConfigurationErrorCLI(t *testing.T) {
	for _, command := range []string{"check", "emit-ir"} {
		_, output, code := runCLIForDiagnostics(t, command, "../../testdata/sema/ipc/command_configuration/valid.sec")
		if code != 0 {
			t.Fatal(command, code, output)
		}
	}
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/ipc/command_configuration/invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, output)
	}
	if doc := decodeOccurrenceDocument(t, output); doc.Summary.Errors != 3 {
		t.Fatal(output)
	}
}
