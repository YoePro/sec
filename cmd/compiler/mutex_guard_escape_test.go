package main

import (
	"strings"
	"testing"
)

// TestMutexGuardExecutionEscapeCLI exposes guard-boundary errors through the
// ordinary check command with source provenance for both sides of a transfer.
// Rules: rules/concurrency/mutex.md — §§39–40,63.
func TestMutexGuardExecutionEscapeCLI(t *testing.T) {
	for _, test := range []struct {
		file         string
		code, errors int
		unproven     bool
	}{
		{"mutex_guard_escape_valid.sec", 0, 0, false},
		{"mutex_guard_escape_invalid.sec", 3, 16, false},
		{"mutex_guard_escape_unproven_invalid.sec", 3, 4, true},
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
			if test.unproven {
				if occurrence.ID == nil || *occurrence.ID != "S1125" || occurrence.Arguments["proof_state"] != "Unproven" {
					t.Fatal(occurrence)
				}
				continue
			}
			if !strings.Contains(occurrence.Message.Text, "bound to the current execution entity") || len(occurrence.Related) == 0 {
				t.Fatal(occurrence)
			}
		}
	}
}
