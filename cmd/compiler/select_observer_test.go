package main

import (
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// TestSelectProcessObserverCLI validates the exact source operation and the
// explicit unsupported IR boundary; no empty-struct observer representation
// or runtime wait is substituted for the missing process lowering.
// Rules: rules/concurrency/select.md §§25–26,54–56; rules/concurrency/processes.md §5.14.
func TestSelectProcessObserverCLI(t *testing.T) {
	const root = "../../testdata/sema/select_observer/"
	if _, output, code := runCLIForDiagnostics(t, "check", root+"valid.sec"); code != 0 {
		t.Fatal(code, output)
	}
	output, stderr, code := runCLIForDiagnostics(t, "emit-ir", root+"valid.sec", "-o", "-")
	if code != 4 || output != "" || !strings.Contains(stderr, "unsupported-lowering") || !strings.Contains(stderr, "ProcessObserver") {
		t.Fatal(code, output, stderr)
	}
	_, output, code = runCLIForDiagnostics(t, "check", root+"invalid.sec", "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, output)
	if code != 3 || doc.Summary.Errors != 9 {
		t.Fatal(code, output)
	}
	counts := map[string]int{}
	for _, d := range doc.Occurrences {
		if d.ID != nil {
			counts[*d.ID]++
		}
	}
	if counts[diagnostics.ObserverWaitUnavailable] != 1 || counts[diagnostics.SelectOperationNotSelectable] != 3 || counts[diagnostics.ProcessObserverInvalidOperation] != 3 {
		t.Fatal(counts, output)
	}
}
