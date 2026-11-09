package main

import (
	"testing"

	"sec/internal/diagnostics"
)

// TestStatementMatchResultsCLI keeps result obligations at the source boundary
// with canonical error identities rather than treating them as lowering gaps.
// Rules: rules/control-flow/flowcontrol_match.md §19(2)-(6);
// rules/tooling/diagnostics.md §26.
func TestStatementMatchResultsCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "sema", "../../testdata/sema/match_statement_results_invalid.sec", "--diagnostic-format=json")
	document := decodeOccurrenceDocument(t, output)
	if code != 3 || document.Summary.Errors != 12 {
		t.Fatalf("code %d: %s", code, output)
	}
	counts := map[string]int{}
	for _, occurrence := range document.Occurrences {
		if occurrence.Severity != diagnostics.SeverityError {
			continue
		}
		if occurrence.ID == nil || occurrence.Unregistered || len(occurrence.Help) == 0 {
			t.Fatalf("incomplete diagnostic: %+v", occurrence)
		}
		counts[*occurrence.ID]++
	}
	if counts[diagnostics.UnhandledMustUseResult] != 11 || counts[diagnostics.NonDiscardableValue] != 1 {
		t.Fatalf("wrong mandatory result diagnostics: %v", counts)
	}
}
