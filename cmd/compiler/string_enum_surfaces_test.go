package main

import (
	"testing"

	"sec/internal/diagnostics"
)

// TestStringEnumFrontendCLI preserves defaults, closed switch coverage and
// duplicate-alias diagnostic identity/provenance through frontend checking.
// Rules: rules/declarations/enums.md — §§7–8,18;
// rules/control-flow/flowcontrol_switch.md — §§20,24.
func TestStringEnumFrontendCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/tooling/string_enum_surfaces/valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/tooling/string_enum_surfaces/duplicate_invalid.sec", "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, output)
	if code != 3 || len(doc.Occurrences) != 1 {
		t.Fatal(code, output)
	}
	d := doc.Occurrences[0]
	if d.ID == nil || *d.ID != diagnostics.DuplicateSwitchCase || d.Primary == nil || d.Primary.Span.Start.Line != 11 || len(d.Related) != 1 || d.Related[0].Span.Start.Line != 9 {
		t.Fatal(d)
	}
}
