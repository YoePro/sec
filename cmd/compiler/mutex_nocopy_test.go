package main

import "testing"

// TestMutexNoCopyCLI validates opaque core declarations on the normal loader
// path and retains copy rejection and explicit move behavior for both owners.
// Rules: rules/concurrency/mutex.md — §§4–6,18,28.
func TestMutexNoCopyCLI(t *testing.T) {
	for _, test := range []struct {
		file         string
		code, errors int
	}{
		{"mutex_nocopy_valid.sec", 0, 0}, {"mutex_nocopy_invalid.sec", 3, 6},
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
