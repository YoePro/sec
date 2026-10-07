package main

import (
	"path/filepath"
	"sec/internal/lexer"
	"sec/internal/sema"
	"strings"
	"testing"
)

// TestEscapeCausePathsLSP checks canonical notes and deduplicated navigation
// against the intermediate source's unsaved text, including UTF-16 columns.
// Rules: rules/analysis/escape_analysis.md — "Cause paths", "Diagnostic quality";
// rules/tooling/lsp.md — "Shared diagnostic model", protocol position encoding.
func TestEscapeCausePathsLSP(t *testing.T) {
	file := filepath.Join(t.TempDir(), "origin.sec")
	current := filepath.Join(t.TempDir(), "return.sec")
	origin := lexer.Token{File: file, Line: 1, Column: 4}
	boundary := lexer.Token{File: current, Line: 1, Column: 1}
	e := sema.Error{File: current, Line: 1, Column: 1, Message: "escape", PreviousFile: file, PreviousLine: 1, PreviousColumn: 4, EscapeCauses: []sema.EscapeCausePath{{Mode: sema.EscapeModeBorrowEscape, Destination: sema.EscapeDestinationReturnedValue, Steps: []sema.EscapeCauseStep{{Kind: "origin", Message: "origin storage is local", Source: origin}, {Kind: "carrier", Message: "carrier", Source: lexer.Token{File: file, Line: 1, Column: 5}}, {Kind: "boundary", Message: "returned dependency", Source: boundary}}}}}
	got := semaDiagnosticWithSources(e, 1, uriFromPath(current), "return", sourceOverlay{file: "😀abc"})
	if len(got.RelatedInformation) != 2 || got.RelatedInformation[0].Location.Range.Start.Character != 4 || got.RelatedInformation[1].Location.Range.Start.Character != 5 {
		t.Fatal(got)
	}
	if !strings.Contains(got.Message, "note: origin storage is local") || !strings.Contains(got.Message, "note: returned dependency") {
		t.Fatal(got)
	}
	e.PreviousLine = 0
	e.EscapeCauses = []sema.EscapeCausePath{{Mode: sema.EscapeModeUnknown, Incomplete: true, Steps: []sema.EscapeCauseStep{{Kind: "unknown", Message: "origin provenance is unknown"}, {Kind: "boundary", Message: "missing proof", Source: boundary}}}}
	got = semaDiagnosticWithSources(e, 1, uriFromPath(current), "return", nil)
	if len(got.RelatedInformation) != 0 || !strings.Contains(got.Message, "cause path is incomplete") {
		t.Fatal(got)
	}
}
