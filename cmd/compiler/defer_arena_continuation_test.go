package main

import (
	"strings"
	"testing"
)

// TestDeferredArenaContinuationCLI verifies registration-time acceptance and
// immediate stale-use rejection survive shared CLI checking and JSON transport.
// Rules: rules/control-flow/defer.md — §§3,9,29;
// rules/memory/arena.md — §§35–36,38,44.
func TestDeferredArenaContinuationCLI(t *testing.T) {
	_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/control_flow/defer_arena_continuation/valid.sec")
	if code != 0 {
		t.Fatal(code, output)
	}
	_, output, code = runCLIForDiagnostics(t, "check", "../../testdata/control_flow/defer_arena_continuation/immediate_invalid.sec", "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, output)
	if code != 3 || len(doc.Occurrences) != 3 {
		t.Fatal(code, output)
	}
	for i, lines := range [][2]int{{7, 5}, {13, 11}, {20, 17}} {
		d := doc.Occurrences[i]
		if d.Primary == nil || d.Primary.Span.Start.Line != lines[0] || len(d.Related) != 1 || d.Related[0].Span.Start.Line != lines[1] || !strings.Contains(d.Message.Text, "cannot use values after arena mem") {
			t.Fatal(d)
		}
	}
}
