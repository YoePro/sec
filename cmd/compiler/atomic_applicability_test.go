package main

import (
	"strings"
	"testing"
)

// TestAtomicOperationApplicabilityCLI checks the same per-T operation surface
// with loader-proven core declarations through the ordinary compiler command.
// Rules: rules/concurrency/atomics.md — §§12–13,27–30,43–45.
func TestAtomicOperationApplicabilityCLI(t *testing.T) {
	for _, test := range []struct {
		file         string
		code, errors int
	}{
		{"atomic_applicability_valid.sec", 0, 0},
		{"atomic_applicability_invalid.sec", 3, 49},
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
			if occurrence.Severity != "error" {
				continue
			}
			if !strings.Contains(occurrence.Message.Text, "not supported for Atomic[") && !strings.Contains(occurrence.Message.Text, "does not take type arguments") {
				t.Fatal(occurrence)
			}
		}
	}
}
