package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMutexReacquisitionLSP presents both acquisitions from compiler diagnostics
// on unsaved source, and clears them after a guard's lifetime ends.
// Rules: rules/concurrency/mutex.md — §§43(1–2),63.
func TestMutexReacquisitionLSP(t *testing.T) {
	uri := uriFromPath(filepath.Join(t.TempDir(), "reacquisition.sec"))
	for _, test := range []struct {
		file   string
		errors int
	}{
		{"mutex_reacquisition_invalid.sec", 12},
		{"mutex_reacquisition_valid.sec", 0},
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
			if !strings.Contains(diagnostic.Message, "already locked by the current execution entity") || len(diagnostic.RelatedInformation) != 1 {
				t.Fatal(diagnostic)
			}
			previous := diagnostic.RelatedInformation[0]
			if previous.Location.URI != uri || previous.Location.Range.Start.Line >= diagnostic.Range.Start.Line || previous.Message != "previous acquisition; guard is still live" {
				t.Fatal(diagnostic)
			}
		}
		if count != test.errors {
			t.Fatal(test.file, count)
		}
	}
}
