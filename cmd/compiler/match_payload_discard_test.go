package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestNonBindingMatchPayloadDiagnosticsCLI preserves the registered mandatory
// discard error at the source boundary instead of accepting invalid match IR.
// Rules: rules/control-flow/flowcontrol_match.md §8(5);
// rules/tooling/diagnostics.md §26.
func TestNonBindingMatchPayloadDiagnosticsCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "sema", "../../testdata/sema/match_payload_discards_invalid.sec", "--diagnostic-format=json")
	document := decodeOccurrenceDocument(t, output)
	if code != 3 || document.Summary.Errors != 17 {
		t.Fatalf("code %d: %s", code, output)
	}
	found := 0
	for _, occurrence := range document.Occurrences {
		if occurrence.Severity != diagnostics.SeverityError {
			continue
		}
		found++
		if occurrence.ID == nil || *occurrence.ID != diagnostics.NonDiscardableValue || occurrence.Unregistered || !strings.Contains(occurrence.Message.Text, "cannot ignore") || len(occurrence.Help) == 0 {
			t.Errorf("incomplete discard diagnostic: %+v", occurrence)
		}
	}
	if found != 17 {
		t.Fatalf("want 17 registered discard errors, got %d", found)
	}
}
