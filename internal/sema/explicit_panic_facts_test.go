package sema

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Explicit panic facts preserve the stable reason, optional static message,
// and exact source provenance needed by Semantic IR and tooling.
//
// Rules:
//   - rules/errors/panic.md — § 13 "Panic information and reason IDs"
//   - rules/errors/panic.md — § 17 "Explicit panic"
func TestResolvedExplicitPanicFactsPreserveCanonicalMetadata(t *testing.T) {
	const path = "explicit-panic-facts.sec"
	result := parser.New(lexer.NewWithFile(`module main

fn WithMessage() void {
	panic "failure"
}

fn Bare() void {
	panic
}
`, path)).Parse()
	if result.HasErrors {
		t.Fatalf("parse diagnostics = %+v", result.Diagnostics)
	}
	analyzer := NewAnalyzer()
	assertSemaErrors(t, analyzer.Analyze(result.Program), nil)

	withStatement := result.Program.Statements[1].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.PanicStatement)
	withMessage, ok := analyzer.ResolvedExplicitPanicOf(withStatement)
	if !ok || withMessage.Reason != PanicReasonExplicitPanic || withMessage.ReasonID != diagnostics.PanicReasonExplicitPanic ||
		!withMessage.HasMessage || withMessage.Message != "failure" || withMessage.File != path ||
		withMessage.Line != 4 || withMessage.Column != 2 || withMessage.Function != "WithMessage" {
		t.Fatalf("message panic fact = %+v", withMessage)
	}

	bareStatement := result.Program.Statements[2].(*ast.FunctionDeclaration).Body.Statements[0].(*ast.PanicStatement)
	bare, ok := analyzer.ResolvedExplicitPanicOf(bareStatement)
	if !ok || bare.Reason != PanicReasonExplicitPanic || bare.ReasonID != diagnostics.PanicReasonExplicitPanic ||
		bare.HasMessage || bare.Message != "" || bare.Function != "Bare" {
		t.Fatalf("bare panic fact = %+v", bare)
	}

	before := len(analyzer.resolvedExplicitPanics)
	if _, ok := analyzer.ResolvedExplicitPanicOf(&ast.PanicStatement{}); ok || len(analyzer.resolvedExplicitPanics) != before {
		t.Fatal("read-only explicit panic query resolved or mutated an unknown statement")
	}
}
