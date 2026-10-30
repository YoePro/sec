package main

import (
	"testing"

	"sec/internal/diagnostics"
)

// TestMatchTerminationCLI checks frontend return/reachability agreement for
// the same closed domains and nested matches used by Sema and the editor.
// Rules: rules/control-flow/flowcontrol_match.md — §23;
// rules/tooling/diagnostics.md — §21.
func TestMatchTerminationCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/control_flow/match_termination/valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	for _, name := range []string{"continuing_invalid", "unreachable_invalid"} {
		_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/control_flow/match_termination/"+name+".sec", "--diagnostic-format=json")
		document := decodeOccurrenceDocument(t, output)
		if code != 3 || len(document.Occurrences) != 6 {
			t.Fatal(name, code, output)
		}
		for _, d := range document.Occurrences {
			if d.Primary == nil || d.Primary.Span.Start.Line == 0 {
				t.Fatal(d)
			}
			if name == "unreachable_invalid" && (d.ID == nil || *d.ID != diagnostics.UnreachableStatement || len(d.Help) == 0) {
				t.Fatal(d)
			}
		}
	}
}
