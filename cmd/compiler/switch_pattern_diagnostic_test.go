package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
)

// TestSwitchStructPatternCLI rejects the source before Sema/backend execution
// and preserves the parser identity, source span and match help in JSON output.
// Rules: rules/control-flow/flowcontrol_switch.md §§28, 40;
// rules/tooling/diagnostics.md §§8–10, 14.
func TestSwitchStructPatternCLI(t *testing.T) {
	fixture := "../../testdata/parser/switch_patterns/qualified_invalid.sec"
	for _, command := range []string{"parse", "sema"} {
		t.Run(command, func(t *testing.T) {
			_, stderr, code := runCLIForDiagnostics(t, command, fixture, "--diagnostic-format", "json")
			document := decodeOccurrenceDocument(t, stderr)
			if code != 2 || len(document.Occurrences) != 1 {
				t.Fatalf("exit = %d, occurrences = %+v", code, document.Occurrences)
			}
			d := document.Occurrences[0]
			if d.ID == nil || *d.ID != diagnostics.ParserInvalidPattern || d.Source != "parser" || d.Severity != diagnostics.SeverityError || d.Primary == nil || d.Primary.Span.File != fixture || d.Primary.Span.Start.Line != 4 || len(d.Help) != 1 || !strings.Contains(d.Help[0].Text, "Use match") {
				t.Fatalf("incorrect structured pattern error: %+v", d)
			}
		})
	}
}
