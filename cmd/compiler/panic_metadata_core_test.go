package main

import "testing"

// Ordinary checks load the real core declaration without a second panic type.
// Rules: rules/errors/panic.md §13; rules/concurrency/tasks.md §11.
func TestPanicMetadataCoreCLI(t *testing.T) {
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/panic_info_valid.sec", "--diagnostic-format=json")
	if code != 0 {
		t.Fatal(code, stderr)
	}
}
