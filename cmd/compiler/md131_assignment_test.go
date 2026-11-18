package main

import (
	"strings"
	"testing"
)

// TestMD131AssignmentCLI verifies the approved rulebook examples: ordinary
// checked mutation is informational, try handles failure, noPanic stays mandatory.
// Rules: rules/types/contracts.md §12(2–3); diagnostics.md §24(4k); panic.md §21.
func TestMD131AssignmentCLI(t *testing.T) {
	for _, mode := range []string{"valid", "invalid"} {
		_, output, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/md131_assignment_"+mode+".sec", "--diagnostic-format=json")
		want := 0
		if mode == "invalid" {
			want = 3
		}
		if code != want || strings.Contains(output, `"S1069"`) {
			t.Fatal(mode, code, output)
		}
		doc := decodeOccurrenceDocument(t, output)
		if doc.Summary.Information != 1 {
			t.Fatal(output)
		}
		for _, item := range doc.Occurrences {
			if item.ID == nil || (*item.ID != "A2011" && *item.ID != "S1092") {
				t.Fatal(item)
			}
		}
	}
}
