package sema

import (
	"reflect"
	"testing"

	"sec/internal/lexer"
)

// TestDiagnosticProvenanceKeepsDomainFacts requires a detached, ordered view
// retaining ordinary source links and every explanation, without inventing a
// source for incomplete or unmapped steps or mutating the owning analyses.
// Rules: rules/compiler/compiler_analysis.md — §8(1–4), §58(2–5).
func TestDiagnosticProvenanceKeepsDomainFacts(t *testing.T) {
	point := lexer.Token{File: "caller.sec", Line: 3, Column: 2}
	e := Error{Related: []RelatedLocation{{File: point.File, Line: point.Line, Column: point.Column, Label: "ownership origin"}},
		EscapeCauses: []EscapeCausePath{{Mode: "borrow-escape", Destination: "returned-value", Incomplete: true,
			Steps: []EscapeCauseStep{{Kind: "origin", Message: "borrowed origin", Source: point}, {Kind: "boundary", Message: "unmapped boundary"}}}},
		AllocationCause: &AllocationCausePath{Unknown: true, Incomplete: true,
			Steps: []AllocationCauseStep{{Kind: "call", Message: "unknown callee", Source: point, Caller: "Root", Callee: "Foreign", Site: "site-1"}}}}
	p := e.DiagnosticProvenance()
	if len(p.Related) != 1 || len(p.Notes) != 5 {
		t.Fatal(p)
	}
	want := []string{"escape.cause.origin", "escape.cause.boundary", "escape.cause.incomplete", "allocation.cause.call", "allocation.cause.incomplete"}
	for i, key := range want {
		if p.Notes[i].Key != key {
			t.Fatal(p)
		}
	}
	if p.Notes[0].Arguments["mode"] != "borrow-escape" || p.Notes[0].Arguments["path"] != "0" || p.Notes[3].Arguments["caller"] != "Root" || p.Notes[3].Arguments["unknown"] != "true" {
		t.Fatal(p)
	}
	for _, i := range []int{1, 2, 4} {
		if p.Notes[i].Source.File != "" || p.Notes[i].Arguments["file"] != "" {
			t.Fatal("invented source", p.Notes[i])
		}
	}
	if !reflect.DeepEqual(p, e.DiagnosticProvenance()) {
		t.Fatal("nondeterministic provenance")
	}
	p.Notes[0].Arguments["mode"] = "changed"
	if p.Notes[1].Arguments["mode"] != "borrow-escape" {
		t.Fatal("steps share mutable arguments")
	}
	p.Notes[0].Message = "changed"
	p.Related[0].Label = "changed"
	actual := e.DiagnosticProvenance()
	if actual.Notes[0].Arguments["mode"] != "borrow-escape" || actual.Notes[0].Message != "borrowed origin" || actual.Related[0].Label != "ownership origin" {
		t.Fatal("provenance aliases owner", actual)
	}
}
