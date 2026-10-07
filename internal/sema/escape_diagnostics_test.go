package sema

import (
	"os"
	"reflect"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
	"strings"
	"testing"
)

// TestEscapeDiagnostics verifies source-level categories, sink/origin locations
// and actionable remedies without changing lifetime decisions.
// Rules: rules/analysis/escape_analysis.md — "Diagnostic categories", "Diagnostic quality".
func TestEscapeDiagnostics(t *testing.T) {
	path := "../../testdata/sema/escape_diagnostics_invalid.sec"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	analyze := func() []Error {
		p := parser.New(lexer.NewWithFile(string(data), path))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		return NewAnalyzer().Analyze(program)
	}
	errs := analyze()
	want := map[string]int{diagnostics.EscapeLocalStorage: 4, diagnostics.EscapeOuterPlace: 1, diagnostics.EscapeMatchPayload: 2, diagnostics.EscapeClosureCapture: 1, diagnostics.EscapeVariadicPack: 1}
	got := map[string]int{}
	for _, e := range errs {
		got[e.ID]++
		if e.Severity != diagnostics.SeverityError || e.Help == "" || e.File != path || e.Line <= 0 || e.Column <= 0 || e.EndLine < e.Line || (e.EndLine == e.Line && e.EndColumn <= e.Column) {
			t.Fatal(e)
		}
		if e.ID != diagnostics.EscapeVariadicPack && (e.PreviousFile != path || e.PreviousLine <= 0 || e.PreviousColumn <= 0) {
			t.Fatal(e)
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("categories %v; errors %v", got, errs)
	}
	if !reflect.DeepEqual(errs, analyze()) {
		t.Fatal("nondeterministic escape diagnostics")
	}
	data, err = os.ReadFile("../../testdata/sema/escape_diagnostics_valid.sec")
	if err != nil {
		t.Fatal(err)
	}
	assertSemaErrors(t, analyzeSourceRaw(t, string(data)), nil)
}

// TestEscapeDiagnosticsUnknownProvenance verifies the conservative provenance
// fallback has its own ID and does not claim a proven local-storage violation.
// Rules: rules/analysis/escape_analysis.md — "Precision exhaustion".
func TestEscapeDiagnosticsUnknownProvenance(t *testing.T) {
	a := NewAnalyzer()
	expr := &ast.Identifier{Token: lexer.Token{Lexeme: "value", File: "unknown.sec", Line: 3, Column: 12}, Value: "value"}
	if !a.checkTrackedReturnedReferenceOrigin("Return", expr, localReferenceOrigin{Unknown: true}) {
		t.Fatal("unknown provenance accepted")
	}
	errs := a.errors
	if len(errs) != 1 || errs[0].ID != diagnostics.EscapeProvenanceUnknown || errs[0].ProofState == diagnostics.ProofInvalid || errs[0].PreviousLine != 0 || !strings.Contains(errs[0].Help, "cannot prove") {
		t.Fatal(errs)
	}
}
