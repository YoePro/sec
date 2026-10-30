package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestInactiveMatchArmCLI reports the MD-042 advisory with one stable
// identity, the default Information severity, the arm pattern location and
// the proof in help, in JSON and human output; the source stays valid.
// Rules: rules/control-flow/flowcontrol_match.md — § 25A;
// rules/tooling/diagnostics.md — § 24(4e).
func TestInactiveMatchArmCLI(t *testing.T) {
	file := "../../testdata/tooling/inactive_match_arm.sec"
	_, stderr, code := runCLIForDiagnostics(t, "check", file, "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, stderr)
	if code != 0 || len(doc.Occurrences) != 1 {
		t.Fatal(code, stderr)
	}
	d := doc.Occurrences[0]
	if d.ID == nil || *d.ID != diagnostics.InactiveMatchArm || d.Name == nil || *d.Name != "control-flow.inactive-match-arm" || d.Unregistered ||
		d.Severity != diagnostics.SeverityInformation || d.Primary == nil || d.Primary.Span.Start.Line != 12 || d.Primary.Span.Start.Column != 9 {
		t.Fatal(stderr)
	}
	help := ""
	for _, message := range d.Help {
		help += message.Text
	}
	if !strings.Contains(help, "figure is proven to contain Figure.Dot") {
		t.Fatal(help)
	}
	_, human, humanCode := runCLIForDiagnostics(t, "check", file)
	if humanCode != 0 || !strings.Contains(human, "this match arm cannot be selected for the current state of figure") {
		t.Fatal(humanCode, human)
	}
}
