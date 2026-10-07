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

// TestEscapeCausePaths checks canonical origin, operations, sink, current
// rebinding, deterministic output and snapshot isolation on Sec source.
// Rules: rules/analysis/escape_analysis.md — "Cause paths", "Diagnostic quality",
// "Current provenance versus historical escape", "Public compiler-facing result".
func TestEscapeCausePaths(t *testing.T) {
	file := "../../testdata/sema/escape_cause_paths_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	parse := func() *Analyzer {
		t.Helper()
		p := parser.New(lexer.NewWithFile(string(data), file))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		a := NewAnalyzer()
		a.Analyze(program)
		return a
	}
	a := parse()
	if len(a.errors) != 3 {
		t.Fatal(a.errors)
	}
	for _, e := range a.errors {
		if e.ID != diagnostics.EscapeLocalStorage || len(e.EscapeCauses) != 1 {
			t.Fatal(e)
		}
		path := e.EscapeCauses[0]
		if path.Mode != EscapeModeBorrowEscape || path.Destination != EscapeDestinationReturnedValue || path.Incomplete || len(path.Steps) < 3 {
			t.Fatalf("%s: %+v", e.Message, path)
		}
		if path.Steps[0].Kind != "origin" || path.Steps[0].Source.Line != e.PreviousLine || path.Steps[len(path.Steps)-1].Kind != "boundary" || path.Steps[len(path.Steps)-1].Source.Line != e.Line {
			t.Fatal(path)
		}
		switch {
		case strings.Contains(e.Message, "ThroughCall"):
			calls, borrows := 0, 0
			for _, step := range path.Steps {
				if step.Kind == "call-summary" {
					calls++
					if step.Source.Line != 4 {
						t.Fatal(step)
					}
				}
				if step.Kind == "borrow" {
					borrows++
				}
			}
			if calls != 1 || borrows != 1 {
				t.Fatal(path)
			}
		case strings.Contains(e.Message, "Projection"):
			projection, carrier := false, false
			for _, step := range path.Steps {
				projection = projection || step.Kind == "projection"
				carrier = carrier || step.Kind == "carrier"
			}
			if !projection || !carrier {
				t.Fatal(path)
			}
		case strings.Contains(e.Message, "Rebound"):
			if path.Steps[0].Source.Line != 16 {
				t.Fatal(path)
			}
			for _, step := range path.Steps {
				if step.Source.Line == 15 {
					t.Fatalf("stale origin in path: %+v", path)
				}
			}
		}
	}
	if !reflect.DeepEqual(a.errors, parse().errors) {
		t.Fatal("nondeterministic explanation")
	}
	published := cloneSemanticErrors(a.errors)
	published[0].EscapeCauses[0].Steps[0].Message = "mutated"
	if a.errors[0].EscapeCauses[0].Steps[0].Message == "mutated" {
		t.Fatal("aliased diagnostic snapshot")
	}
}

// TestEscapeCausePathCategories covers every represented diagnostic category,
// the unknown proof fallback and duplicate-report suppression.
// Rules: rules/analysis/escape_analysis.md — "Cause paths", "Diagnostic architecture", "Unknown escape".
func TestEscapeCausePathCategories(t *testing.T) {
	file := "../../testdata/sema/escape_diagnostics_invalid.sec"
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.NewWithFile(string(data), file))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	errs := a.Analyze(program)
	if len(errs) != 9 {
		t.Fatal(errs)
	}
	for _, e := range errs {
		if len(e.EscapeCauses) != 1 || len(e.EscapeCauses[0].Steps) < 2 {
			t.Fatalf("%s: %+v", e.Message, e.EscapeCauses)
		}
		path := e.EscapeCauses[0]
		if path.Incomplete || path.Steps[0].Kind != "origin" {
			t.Fatal(path)
		}
		if e.ID == diagnostics.EscapeOuterPlace && path.Destination != EscapeDestinationOuterPlace {
			t.Fatal(path)
		}
		if e.ID == diagnostics.EscapeClosureCapture && path.Mode != EscapeModeCapture {
			t.Fatal(path)
		}
	}
	a = NewAnalyzer()
	token := lexer.Token{File: "unknown.sec", Line: 3, Column: 1}
	a.reportEscapeDiagnostic(token, lexer.Token{}, diagnostics.EscapeProvenanceUnknown, "unknown")
	a.reportEscapeDiagnostic(token, lexer.Token{}, diagnostics.EscapeProvenanceUnknown, "unknown")
	if len(a.errors) != 1 {
		t.Fatal(a.errors)
	}
	path := a.errors[0].EscapeCauses[0]
	if !path.Incomplete || path.Mode != EscapeModeUnknown || path.Steps[0].Kind != "unknown" || path.Steps[0].Source.Line != 0 {
		t.Fatal(path)
	}
}

// TestEscapeFactCausePaths verifies lazy canonical source, carrier and Place
// explanations remain deterministic, detached and explicit about uncertainty.
// Rules: rules/analysis/escape_analysis.md — "Cause paths", "Place provenance", "Public compiler-facing result".
func TestEscapeFactCausePaths(t *testing.T) {
	fact := EscapeFact{Mode: EscapeModeBorrowEscape, Destination: EscapeDestinationReturnedValue, Sink: lexer.Token{File: "fact.sec", Line: 10, Column: 1}, Sources: []EscapeSource{{Kind: EscapeSourceLocal, Name: "later", Token: lexer.Token{File: "fact.sec", Line: 3, Column: 1}, CarrierPath: ".body"}, {Kind: EscapeSourceLocal, Name: "earlier", Token: lexer.Token{File: "fact.sec", Line: 2, Column: 1}}}}
	paths := fact.CausePaths()
	if len(paths) != 2 || paths[0].Steps[0].Source.Line != 2 || paths[1].Steps[1].Kind != "carrier" {
		t.Fatal(paths)
	}
	paths[0].Steps[0].Message = "mutated"
	if reflect.DeepEqual(paths, fact.CausePaths()) {
		t.Fatal("aliased cause paths")
	}
	if fact.Sources[0].Name != "later" {
		t.Fatal("source facts sorted in place")
	}
	fact.Sources[0].Place = Place{Root: "later", Projections: []PlaceProjection{{Kind: PlaceField, Name: "value", Token: lexer.Token{File: "fact.sec", Line: 3, Column: 2}}}}
	paths = fact.CausePaths()
	if paths[1].Steps[1].Kind != "projection" || paths[1].Steps[1].Source.Column != 2 {
		t.Fatal(paths)
	}
	fact.Unknown = true
	if !fact.CausePaths()[0].Incomplete {
		t.Fatal("unknown fact lost")
	}
	fact.Sources = nil
	paths = fact.CausePaths()
	if len(paths) != 1 || !paths[0].Incomplete || paths[0].Steps[0].Kind != "unknown" {
		t.Fatal(paths)
	}
}

// TestEscapeCausePathBound checks bounded reconstruction does not invent
// an unrelated field's origin or change mandatory diagnostics at exhaustion.
// Rules: rules/analysis/escape_analysis.md — "Bounded precision", "Place provenance", "Cause paths".
func TestEscapeCausePathBound(t *testing.T) {
	a := NewAnalyzer()
	origin := lexer.Token{File: "bound.sec", Line: 1, Column: 1}
	var expr ast.Expression = &ast.Identifier{Value: "value", Token: origin}
	for i := 0; i < 100; i++ {
		expr = &ast.RefExpression{Value: expr, Token: lexer.Token{File: "bound.sec", Line: 2, Column: 1}}
	}
	a.reportEscapeExpressionDiagnostic(expr, origin, diagnostics.EscapeLocalStorage, "local")
	path := a.errors[0].EscapeCauses[0]
	if !path.Incomplete || len(path.Steps) > 64 || path.Steps[len(path.Steps)-1].Kind != "boundary" {
		t.Fatal(path)
	}
	a.localRefContainers = make(map[string]localReferenceOrigin)
	a.localRefContainers["carrier"] = localReferenceOrigin{Contained: map[string]localReferenceOrigin{".left": {Token: origin}, ".right": {Token: lexer.Token{File: "bound.sec", Line: 3, Column: 1}}}}
	field := &ast.MemberExpression{Object: &ast.Identifier{Value: "carrier"}, Property: &ast.Identifier{Value: "right"}}
	if a.escapeExpressionHasOrigin(field, origin) {
		t.Fatal("selected another field's origin")
	}
}

// TestEscapeCausePathCallArgument checks equal origins cannot fabricate a
// dependency through a parameter absent from the canonical return summary.
// Rules: rules/analysis/escape_analysis.md — "Calls", "Cause paths", "Place provenance".
func TestEscapeCausePathCallArgument(t *testing.T) {
	a := NewAnalyzer()
	origin := lexer.Token{File: "args.sec", Line: 1, Column: 1}
	unused := &ast.RefExpression{Value: &ast.Identifier{Token: origin}, Token: lexer.Token{File: "args.sec", Line: 2, Column: 1}}
	used := &ast.RefExpression{Value: &ast.Identifier{Token: origin}, Token: lexer.Token{File: "args.sec", Line: 3, Column: 1}}
	call := &ast.CallExpression{Token: lexer.Token{File: "args.sec", Line: 4, Column: 1}, Arguments: []ast.Expression{unused, used}}
	a.resolvedCalls = map[*ast.CallExpression]ResolvedCall{call: {Function: Function{Name: "Select", HasReturnOrigin: true, ReturnOrigin: localReferenceOrigin{HasPlace: true, Place: Place{Root: "$param:1"}}}}}
	a.reportEscapeExpressionDiagnostic(call, origin, diagnostics.EscapeLocalStorage, "local")
	path := a.errors[0].EscapeCauses[0]
	found := false
	for _, step := range path.Steps {
		if step.Kind == "borrow" {
			if step.Source.Line == 2 {
				t.Fatal("unused argument included", path)
			}
			found = step.Source.Line == 3
		}
	}
	if !found {
		t.Fatal(path)
	}
}
