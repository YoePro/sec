package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestAtomicOrderMatrixCLI validates orders with the real core declaration
// loaded and reports violations during check, without backend validation.
// Rules: rules/concurrency/atomics.md — §§14–25,45(5–8).
func TestAtomicOrderMatrixCLI(t *testing.T) {
	for _, test := range []struct {
		file         string
		code, errors int
	}{
		{"atomic_orders_valid.sec", 0, 0},
		{"atomic_orders_invalid.sec", 3, 43},
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
		invalid, wrongType, runtime, execution := 0, 0, 0, 0
		for _, occurrence := range document.Occurrences {
			if occurrence.Severity != "error" {
				continue
			}
			if strings.Contains(occurrence.Message.Text, "is not valid for") {
				invalid++
			}
			if strings.Contains(occurrence.Message.Text, "must be MemoryOrder") {
				wrongType++
			}
			if occurrence.ID != nil && *occurrence.ID == diagnostics.AtomicOrderNotCompileTime {
				runtime++
			}
			if occurrence.ID != nil && *occurrence.ID == diagnostics.SemanticCompileTimeExecutionUnavailable {
				execution++
			}
		}
		if invalid != 17 || wrongType != 10 || runtime != 12 || execution != 2 {
			t.Fatal(stderr)
		}
	}
}
