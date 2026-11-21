package main

import (
	"sec/internal/diagnostics"
	"testing"
)

// TestDeferFallibleHandlingCLI preserves cleanup-local repairs, source spans
// and stable mandatory diagnostic identities in the public JSON transport.
// Rules: rules/control-flow/defer.md §15; rules/tooling/diagnostics.md §§14–15,26.
func TestDeferFallibleHandlingCLI(t *testing.T) {
	const root = "../../testdata/control_flow/defer_fallible/"
	if _, output, code := runCLIForDiagnostics(t, "check", root+"valid.sec"); code != 0 {
		t.Fatal(code, output)
	}
	_, output, code := runCLIForDiagnostics(t, "check", root+"invalid.sec", "--diagnostic-format=json")
	document := decodeOccurrenceDocument(t, output)
	if code != 3 || document.Summary.Errors != 7 {
		t.Fatal(code, output)
	}
	counts := map[string]int{}
	for _, d := range document.Occurrences {
		if d.Severity != diagnostics.SeverityError {
			continue
		}
		if d.ID == nil || d.Unregistered || len(d.Help) == 0 || d.Primary == nil || d.Primary.Span.Start.Line == 0 {
			t.Fatalf("incomplete diagnostic: %+v", d)
		}
		counts[*d.ID]++
	}
	if counts[diagnostics.UnhandledMustUseResult] != 1 || counts[diagnostics.TryPropagationIncompatible] != 4 || counts[diagnostics.TryResidualUnpropagatable] != 2 {
		t.Fatal(counts)
	}
}
