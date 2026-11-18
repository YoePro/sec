package main

import (
	"strings"
	"testing"
)

// TestAnalyseChildPanicDomains publishes the shared domain facts for exact
// callable focus and preserves synchronous argument failures under noPanic.
// Rules: rules/errors/panic.md §§6–9,21,28(7); cli.md §5.3.
func TestAnalyseChildPanicDomains(t *testing.T) {
	const valid = "../../testdata/sema/child_panic_domains_valid.sec"
	for _, name := range []string{"SpawnTask", "SpawnThread", "SpawnLambda", "SpawnThreadLambda", "SpawnValue", "SpawnGeneric", "SpawnCaptured", "SpawnForeign", "Top"} {
		out, errText, code := runCLIForDiagnostics(t, "analyse", "--verbose", "-f", name, valid)
		if code != 0 || errText != "" || !strings.Contains(out, name+": Child panic domain spawn-") || !strings.Contains(out, "containment follows target policy") || strings.Contains(out, name+": may panic via") {
			t.Fatal(name, code, out, errText)
		}
		if name == "SpawnForeign" && !strings.Contains(out, "panic behavior unproven") {
			t.Fatal(out)
		}
	}
	out, errText, code := runCLIForDiagnostics(t, "check", "../../testdata/sema/child_panic_domains_invalid.sec", "--diagnostic-format=json")
	if code != 3 {
		t.Fatal(code, out, errText)
	}
	doc := decodeOccurrenceDocument(t, errText)
	if doc.Summary.Errors != 2 {
		t.Fatal(errText)
	}
	for _, item := range doc.Occurrences {
		if item.ID == nil || *item.ID != "S1092" {
			t.Fatal(item)
		}
	}
}
