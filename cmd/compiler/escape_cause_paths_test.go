package main

import (
	"strings"
	"testing"
)

// TestEscapeCausePathsCLI checks ordered JSON causes, canonical mode and
// destination, intermediate source links and the equivalent human explanation.
// Rules: rules/analysis/escape_analysis.md — "Cause paths", "Diagnostic quality";
// rules/tooling/diagnostics.md — §§8–10, 14.
func TestEscapeCausePathsCLI(t *testing.T) {
	path := "../../testdata/sema/escape_cause_paths_invalid.sec"
	_, output, code := runCLIForDiagnostics(t, "sema", path, "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, output)
	if code != 3 || doc.Summary.Errors != 3 || len(doc.Occurrences) != 3 {
		t.Fatal(code, output)
	}
	for _, e := range doc.Occurrences {
		if len(e.Notes) < 3 || e.Notes[0].Key != "escape.cause.origin" || e.Notes[len(e.Notes)-1].Key != "escape.cause.boundary" {
			t.Fatal(e)
		}
		for _, note := range e.Notes {
			if note.Arguments["mode"] != "borrow-escape" || note.Arguments["destination"] != "returned-value" || note.Arguments["incomplete"] != "false" || note.Arguments["file"] != path {
				t.Fatal(note)
			}
		}
		if strings.Contains(e.Message.Text, "ThroughCall") {
			found := false
			for _, related := range e.Related {
				if related.Span.Start.Line == 4 && related.Message != nil && related.Message.Key == "escape.cause.call-summary" {
					found = true
				}
			}
			if !found {
				t.Fatal(e)
			}
		}
	}
	_, human, humanCode := runCLIForDiagnostics(t, "sema", path)
	if humanCode != code || !strings.Contains(human, "note: resolved return summary of Identity") || !strings.Contains(human, "note: returned dependency requires storage") {
		t.Fatal(humanCode, human)
	}
}
