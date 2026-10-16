package main

import "testing"

// TestMutexCanonicalNamingCLI verifies that public spelling checks and the
// nonblocking TryLock guarantee are shared by the compiler command path.
// Rules: rules/concurrency/mutex.md — §§3,20–22.
func TestMutexCanonicalNamingCLI(t *testing.T) {
	for _, test := range []struct {
		file         string
		code, errors int
	}{
		{"mutex_naming_valid.sec", 0, 0}, {"mutex_naming_invalid.sec", 3, 2},
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
