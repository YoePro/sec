package main

import "testing"

// TestMutexAcquisitionOverloadsCLI validates the same signature matrix with
// real core loading, including ContextError and contextual duration conversion.
// Rules: rules/concurrency/mutex.md — §§7–8,11–12,20–24.
func TestMutexAcquisitionOverloadsCLI(t *testing.T) {
	for _, test := range []struct {
		file         string
		code, errors int
	}{
		{"mutex_overloads_valid.sec", 0, 0}, {"mutex_overloads_invalid.sec", 3, 11},
	} {
		_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/"+test.file, "--diagnostic-format=json")
		if code != test.code {
			t.Fatal(test.file, code, stderr)
		}
		if test.errors > 0 && decodeOccurrenceDocument(t, stderr).Summary.Errors != test.errors {
			t.Fatal(stderr)
		}
	}
}
