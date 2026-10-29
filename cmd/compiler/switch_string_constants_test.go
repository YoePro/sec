package main

import (
	"testing"

	"sec/internal/diagnostics"
)

// TestComputedStringSwitchCLI preserves the shared constant proof, diagnostic
// identity, actionable help and earlier-case provenance through CLI checking.
// Rules: rules/control-flow/flowcontrol_switch.md — §§20,25;
// rules/tooling/diagnostics.md — §§8–10.
func TestComputedStringSwitchCLI(t *testing.T) {
	_, stderr, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/switch_string_constants/valid.sec")
	if code != 0 {
		t.Fatal(code, stderr)
	}
	fixture := "../../testdata/sema/switch_string_constants/duplicate_invalid.sec"
	_, stderr, code = runCLIForDiagnostics(t, "check", fixture, "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, stderr)
	if code != 3 || len(doc.Occurrences) != 3 {
		t.Fatal(code, stderr)
	}
	for i, lines := range [][2]int{{6, 5}, {11, 11}, {17, 16}} {
		d := doc.Occurrences[i]
		if d.ID == nil || *d.ID != diagnostics.DuplicateSwitchCase || d.Unregistered || d.Severity != diagnostics.SeverityError || d.Primary == nil || d.Primary.Span.File != fixture || d.Primary.Span.Start.Line != lines[0] || len(d.Help) != 1 || len(d.Related) != 1 {
			t.Fatalf("incorrect duplicate diagnostic: %+v", d)
		}
		if d.Related[0].Span.Start.Line != lines[1] || d.Related[0].Span.File != fixture || d.Related[0].Message.Text != "earlier equivalent string case" {
			t.Fatalf("incorrect duplicate provenance: %+v", d)
		}
	}
}
