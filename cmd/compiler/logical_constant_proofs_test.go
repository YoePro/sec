package main

import (
	"testing"

	"sec/internal/diagnostics"
)

// TestSharedConstantConditionCLI preserves Sema's branch proofs, mandatory
// unreachable identity, source ranges and practical help in CLI JSON output.
// Rules: rules/control-flow/flowcontrol_if.md — §20;
// rules/control-flow/flowcontrol_while.md — §19; rules/tooling/diagnostics.md — §21.
func TestSharedConstantConditionCLI(t *testing.T) {
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/control_flow/logical_constants/valid.sec")
	if code != 0 {
		t.Fatal(code, stderr)
	}
	fixture := "../../testdata/control_flow/logical_constants/unreachable_invalid.sec"
	_, stderr, code = runCLIForDiagnostics(t, "check", fixture, "--diagnostic-format=json")
	document := decodeOccurrenceDocument(t, stderr)
	if code != 3 || len(document.Occurrences) != 2 {
		t.Fatal(code, stderr)
	}
	for i, line := range []int{5, 8} {
		d := document.Occurrences[i]
		if d.ID == nil || *d.ID != diagnostics.UnreachableStatement || d.Unregistered || d.Severity != diagnostics.SeverityError || d.Primary == nil || d.Primary.Span.Start.Line != line || d.Primary.Span.File != fixture || len(d.Help) != 1 {
			t.Fatal(d)
		}
	}
}
