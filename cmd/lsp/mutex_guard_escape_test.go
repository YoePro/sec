package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestMutexGuardExecutionEscapeLSP shares compiler escape diagnostics for
// unsaved source and clears them when transfers stay in the current execution.
// Rules: rules/concurrency/mutex.md — §§39–40,63.
func TestMutexGuardExecutionEscapeLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "guard_escape.sec"))
	for _, test := range []struct {
		file     string
		errors   int
		unproven bool
	}{
		{"mutex_guard_escape_invalid.sec", 16, false},
		{"mutex_guard_escape_unproven_invalid.sec", 4, true},
		{"mutex_guard_escape_valid.sec", 0, false},
	} {
		data, err := os.ReadFile("../../testdata/sema/" + test.file)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, diagnostic := range analyze(uri, string(data)) {
			if diagnostic.Severity != 1 {
				continue
			}
			count++
			if test.unproven {
				if diagnostic.Code != diagnostics.RequiredProofUnavailable || diagnostic.Data == nil || diagnostic.Data.ProofState != diagnostics.ProofUnproven {
					t.Fatal(diagnostic)
				}
				continue
			}
			if !strings.Contains(diagnostic.Message, "bound to the current execution entity") || len(diagnostic.RelatedInformation) != 1 {
				t.Fatal(diagnostic)
			}
			previous := diagnostic.RelatedInformation[0]
			if previous.Location.URI != uri || previous.Message != "execution-bound guard origin" {
				t.Fatal(diagnostic)
			}
		}
		if count != test.errors {
			t.Fatal(test.file, count)
		}
	}
}
