package main

import (
	"strings"
	"testing"
)

// TestMutexReacquisitionCLI keeps non-reentrancy a compiler error, independently
// of optional deep analyses or editor policy.
// Rules: rules/concurrency/mutex.md — §§42–43,63.
func TestMutexReacquisitionCLI(t *testing.T) {
	for _, test := range []struct {
		file         string
		code, errors int
	}{
		{"mutex_reacquisition_valid.sec", 0, 0},
		{"mutex_reacquisition_invalid.sec", 3, 12},
	} {
		_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/"+test.file, "--diagnostic-format=json")
		if code != test.code {
			t.Fatal(test.file, code, stderr)
		}
		if test.errors == 0 {
			continue
		}
		document := decodeOccurrenceDocument(t, stderr)
		if document.Summary.Errors != test.errors {
			t.Fatal(stderr)
		}
		for _, occurrence := range document.Occurrences {
			if occurrence.Severity == "error" && !strings.Contains(occurrence.Message.Text, "already locked by the current execution entity") {
				t.Fatal(occurrence)
			}
		}
	}
}
