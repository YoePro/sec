package main

import (
	"bytes"
	"strings"
	"testing"

	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/sema"
)

// TestDiagnosticProvenanceMixedCLI keeps one primary owner and transports both
// domains' ordered notes, metadata and incomplete markers in human and JSON.
// Rules: rules/compiler/compiler_analysis.md — §8(1–4), §58(2–5), §59(1–3).
func TestDiagnosticProvenanceMixedCLI(t *testing.T) {
	point := lexer.Token{File: "helper.sec", Line: 1, Column: 2}
	e := sema.Error{ID: diagnostics.RequiredProofUnavailable, Severity: diagnostics.SeverityError, ProofState: diagnostics.ProofUnproven,
		Message: "required proof missing", File: "root.sec", Line: 1, Column: 1,
		PreviousFile: point.File, PreviousLine: point.Line, PreviousColumn: point.Column, RelatedLabel: "ownership origin",
		EscapeCauses: []sema.EscapeCausePath{{Mode: "borrow-escape", Destination: "returned-value", Incomplete: true,
			Steps: []sema.EscapeCauseStep{{Kind: "origin", Source: point, Message: "borrowed origin"}}}},
		AllocationCause: &sema.AllocationCausePath{Unknown: true, Incomplete: true,
			Steps: []sema.AllocationCauseStep{{Kind: "call", Source: point, Message: "unknown callee", Caller: "Root", Callee: "Foreign"}}}}
	var output bytes.Buffer
	r := &diagnosticReporter{format: diagnosticFormatJSON, output: &output}
	r.semaDiagnostic(e, "root occurrence\n")
	r.finish()
	doc := decodeOccurrenceDocument(t, output.String())
	if len(doc.Occurrences) != 1 {
		t.Fatal(doc)
	}
	item := doc.Occurrences[0]
	if item.ID == nil || *item.ID != e.ID || item.Arguments["proof_state"] != "Unproven" || len(item.Related) != 1 || item.Related[0].Message.Text != "ownership origin" || len(item.Notes) != 4 {
		t.Fatal(item)
	}
	for i, note := range e.DiagnosticProvenance().Notes {
		if item.Notes[i].Key != note.Key || item.Notes[i].Arguments["incomplete"] != "true" || !strings.Contains(item.Notes[i].Text, note.Message) {
			t.Fatal(item.Notes[i], note)
		}
	}
	output.Reset()
	r = &diagnosticReporter{format: diagnosticFormatHuman, output: &output}
	r.semaDiagnostic(e, "root occurrence\n")
	for _, note := range e.DiagnosticProvenance().Notes {
		if !strings.Contains(output.String(), "note: "+note.Message) {
			t.Fatal(output.String(), note)
		}
	}
}
