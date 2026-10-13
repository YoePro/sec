package main

import (
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// TestDiagnosticProvenanceMixedLSP preserves the common owner notes, incomplete
// markers and ordinary provenance, mapping related overlay text to UTF-16 and
// retaining one navigation link when different domains share a source point.
// Rules: rules/compiler/compiler_analysis.md — §8(1–4), §58(2–5), §59(1–3);
// rules/tooling/lsp.md — Shared diagnostic model, protocol position encoding.
func TestDiagnosticProvenanceMixedLSP(t *testing.T) {
	point := lexer.Token{File: "/tmp/provenance-helper.sec", Line: 1, Column: 2}
	e := sema.Error{ID: diagnostics.RequiredProofUnavailable, ProofState: diagnostics.ProofUnproven,
		Message: "required proof missing", File: "/tmp/provenance-root.sec", Line: 1, Column: 1,
		PreviousFile: point.File, PreviousLine: point.Line, PreviousColumn: point.Column, RelatedLabel: "ownership origin",
		EscapeCauses: []sema.EscapeCausePath{{Mode: "borrow-escape", Incomplete: true,
			Steps: []sema.EscapeCauseStep{{Kind: "origin", Source: point, Message: "borrowed origin"}}}},
		AllocationCause: &sema.AllocationCausePath{Unknown: true, Incomplete: true,
			Steps: []sema.AllocationCauseStep{{Kind: "call", Source: point, Message: "unknown callee"}}}}
	got := semaDiagnosticWithSources(e, 3, uriFromPath(e.File), "x", sourceOverlay{Sources: map[string]string{point.File: "𐐀x"}})
	if got.Code != e.ID || got.Severity != 1 || got.Data == nil || got.Data.ProofState != e.ProofState || len(got.RelatedInformation) != 1 {
		t.Fatal(got)
	}
	if link := got.RelatedInformation[0]; link.Message != "ownership origin" || link.Location.Range.Start.Character != 2 {
		t.Fatal(link)
	}
	previous := -1
	for _, note := range e.DiagnosticProvenance().Notes {
		index := strings.Index(got.Message, "note: "+note.Message)
		if index <= previous {
			t.Fatal("missing or reordered note", note, got)
		}
		previous = index
	}
}
