package main

import (
	"strings"
	"testing"

	"sec/internal/lexer"
	"sec/internal/sema"
)

// TestAllocationCausePathsCLI verifies ordered human/JSON witnesses with one
// owning S1108 occurrence and deduplicated navigable terminal/call-site links.
// Rules: rules/memory/allocation.md — §§28(4),29(4); rules/tooling/diagnostics.md — §§8–10,14.
func TestAllocationCausePathsCLI(t *testing.T) {
	file := "../../testdata/sema/allocation_cause_paths_invalid.sec"
	_, output, code := runCLIForDiagnostics(t, "sema", file, "--diagnostic-format=json")
	doc := decodeOccurrenceDocument(t, output)
	if code != 3 || doc.Summary.Errors != 4 || len(doc.Occurrences) != 4 {
		t.Fatal(code, output)
	}
	for _, e := range doc.Occurrences {
		if e.ID == nil || *e.ID != "S1108" || len(e.Notes) < 3 || e.Notes[0].Key != "allocation.cause.root" || e.Notes[len(e.Notes)-1].Key != "allocation.cause.operation" {
			t.Fatal(e)
		}
		for _, note := range e.Notes {
			if note.Arguments["file"] != file || note.Arguments["incomplete"] != "false" {
				t.Fatal(note)
			}
		}
		seen := map[occurrencePosition]bool{}
		for _, related := range e.Related {
			if seen[related.Span.Start] {
				t.Fatal("duplicate navigation", e)
			}
			seen[related.Span.Start] = true
		}
	}
	_, analysisJSON, analysisCode := runCLIForDiagnostics(t, "analyse", file, "--diagnostic-format=json")
	analysis := decodeOccurrenceDocument(t, analysisJSON)
	if analysisCode != code || analysis.Summary.Errors != doc.Summary.Errors {
		t.Fatal("analyse lost owning violations", analysisCode, analysisJSON)
	}
	for _, occurrence := range analysis.Occurrences {
		if occurrence.ID == nil || *occurrence.ID != "S1108" || len(occurrence.Notes) < 3 || occurrence.Notes[len(occurrence.Notes)-1].Key != "allocation.cause.operation" {
			t.Fatal("analyse lost allocation witness", occurrence)
		}
	}
	_, human, humanCode := runCLIForDiagnostics(t, "sema", file)
	if humanCode != code || !strings.Contains(human, "note: Middle reaches Allocate") || !strings.Contains(human, "note: unresolved allocation behavior:") {
		t.Fatal(humanCode, human)
	}
}

// TestAllocationCauseMissingSource preserves incomplete evidence as notes
// without fabricating a related link when a source file is unavailable.
// Rules: rules/memory/allocation.md — §§24(6),29(4).
func TestAllocationCauseMissingSource(t *testing.T) {
	occurrence := emittedOccurrence{}
	e := sema.Error{AllocationCause: &sema.AllocationCausePath{Incomplete: true, Steps: []sema.AllocationCauseStep{{Kind: "operation", Source: lexer.Token{Line: 2, Column: 3}, Message: "unavailable operation source"}}}}
	human := appendAllocationCausePath(&occurrence, e, "")
	if len(occurrence.Related) != 0 || len(occurrence.Notes) != 1 || !strings.Contains(human, "allocation cause path is incomplete") {
		t.Fatal(occurrence, human)
	}
}
