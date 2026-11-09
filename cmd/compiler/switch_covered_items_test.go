package main

import (
	"sec/internal/diagnostics"
	"testing"
)

// TestCoveredSwitchItemsCLI preserves registered coverage identities and proof
// locations through compiler JSON presentation.
// Rules: rules/control-flow/flowcontrol_switch.md §34;
// rules/tooling/diagnostics.md §§14,21.
func TestCoveredSwitchItemsCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "sema", "../../testdata/sema/switch_covered_items_invalid.sec", "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, output)
	if code != 3 || doc.Summary.Errors != 10 {
		t.Fatalf("code %d: %s", code, output)
	}
	for _, d := range doc.Occurrences {
		if d.Severity != diagnostics.SeverityError {
			continue
		}
		if d.ID == nil || (*d.ID != diagnostics.UnreachableStatement && *d.ID != diagnostics.DuplicateSwitchCase) || d.Unregistered || len(d.Help) == 0 || len(d.Related) == 0 {
			t.Errorf("incomplete covered-case diagnostic: %+v", d)
		}
	}
}
