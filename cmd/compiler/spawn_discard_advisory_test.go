package main

import (
	"sec/internal/diagnostics"
	"strings"
	"testing"
)

// CLI JSON and human diagnostics preserve A3001 and its source provenance,
// while explaining successful child handles separately from raw spawn results.
// Rules: rules/tooling/diagnostics.md §25(8); rules/concurrency/spawn.md §3A(8).
func TestSpawnDiscardAdvisoryCLI(t *testing.T) {
	file := "../../testdata/sema/spawn_blocks/advice.sec"
	_, output, code := runCLIForDiagnostics(t, "check", file, "--diagnostic-format=json")
	if code != 0 {
		t.Fatal(code, output)
	}
	doc := decodeOccurrenceDocument(t, output)
	count := 0
	for _, d := range doc.Occurrences {
		if d.ID == nil || *d.ID != diagnostics.ImplicitDiscardedResult {
			t.Fatal(d)
		}
		if d.Severity != diagnostics.SeverityInformation || d.Primary == nil || len(d.Help) == 0 {
			t.Fatal(d)
		}
		if strings.Contains(d.Message.Text, "successful handle") {
			count++
		}
	}
	if count != 5 || len(doc.Occurrences) != 7 {
		t.Fatal(output)
	}
	_, human, code := runCLIForDiagnostics(t, "check", file)
	if code != 0 || !strings.Contains(human, "successful handle is Task[void]") || !strings.Contains(human, "successful handle is Thread[void]") {
		t.Fatal(code, human)
	}
}
