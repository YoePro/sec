package main

import (
	"strings"
	"testing"
)

// TestThreadMetadataBoundariesCLI verifies canonical metadata and observer-only
// behavior through compiler diagnostics without enabling thread runtime paths.
// Rules: rules/concurrency/threads.md — §§23,40,43.
func TestThreadMetadataBoundariesCLI(t *testing.T) {
	for _, area := range []string{"thread_termination", "thread_observer_wait"} {
		_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/"+area+"_valid.sec")
		if code != 0 {
			t.Fatal(code, output)
		}
		_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/sema/"+area+"_invalid.sec", "--diagnostic-format=json")
		if code != 3 {
			t.Fatal(code, output)
		}
		doc := decodeOccurrenceDocument(t, output)
		if doc.Summary.Errors != 3 {
			t.Fatal(output)
		}
		if area == "thread_observer_wait" && !strings.Contains(output, "no wait or selectable completion capability") {
			t.Fatal(output)
		}
	}
}
