package main

import (
	"strings"
	"testing"
)

// TestCommandIOModesCLI verifies real core mode declarations, nominal non-error
// typing, exhaustive matching and Inherit defaults. Semantic IR retains Resource
// as its own member of each enum; it establishes no actual process I/O binding.
// Rules: rules/concurrency/ipc.md — §94;
// rules/concurrency/processes.md — §§5.12–5.13.
func TestCommandIOModesCLI(t *testing.T) {
	for _, fixture := range []struct {
		name         string
		code, errors int
	}{
		{"command_modes_valid.sec", 0, 0}, {"command_modes_invalid.sec", 3, 8}, {"command_modes_spoof_invalid.sec", 3, 2},
	} {
		_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/ipc/"+fixture.name, "--diagnostic-format=json")
		if code != fixture.code {
			t.Fatal(fixture.name, code, stderr)
		}
		if fixture.errors != 0 && decodeOccurrenceDocument(t, stderr).Summary.Errors != fixture.errors {
			t.Fatal(stderr)
		}
	}
	stdout, stderr, code := runCLIForDiagnostics(t, "emit-ir", "../../testdata/sema/ipc/command_modes_valid.sec")
	if code != 0 {
		t.Fatal(code, stderr)
	}
	for _, name := range []string{"core::CommandInputMode", "core::CommandOutputMode", "Resource", "Inherit", "enum.constant", "enum.compare"} {
		if !strings.Contains(stdout, name) {
			t.Fatal(name, stdout)
		}
	}
}
