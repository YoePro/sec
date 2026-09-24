package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestResolvedAssertionFactsPreserveCanonicalMetadata verifies that valid
// assertions publish their stable reason, static message, source provenance,
// enclosing function, and proof state through the read-only Sema API.
//
// Rules:
//   - rules/errors/panic.md — § 13(1)–(3) "Panic information and reason IDs"
//   - rules/errors/panic.md — §§ 15.3–15.5 "Meaning", "Messages", "Assertions are always active"
func TestResolvedAssertionFactsPreserveCanonicalMetadata(t *testing.T) {
	path := "../../testdata/sema/assertion_facts_valid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := parser.New(lexer.NewWithFile(string(source), path)).Parse()
	if result.HasErrors {
		t.Fatalf("parse diagnostics = %+v", result.Diagnostics)
	}
	analyzer := NewAnalyzer()
	assertSemaErrors(t, analyzer.Analyze(result.Program), nil)

	function := result.Program.Statements[1].(*ast.FunctionDeclaration)
	assertions := []*ast.AssertStatement{
		function.Body.Statements[0].(*ast.AssertStatement),
		function.Body.Statements[1].(*ast.AssertStatement),
	}
	withMessage, ok := analyzer.ResolvedAssertionOf(assertions[0])
	if !ok || withMessage.Reason != PanicReasonAssertionFailed || withMessage.ReasonID != diagnostics.PanicReasonAssertionFailure || !withMessage.HasMessage ||
		withMessage.Message != "ready required" || withMessage.File != path ||
		withMessage.Line != 4 || withMessage.Column != 5 || withMessage.Function != "Check" || withMessage.Proven {
		t.Fatalf("message assertion fact = %+v", withMessage)
	}
	assertionCondition, ok := analyzer.ResolvedConditionFactOf(assertions[0].Condition)
	if !ok || assertionCondition.Kind != ConditionFactAssertionSuccess || assertionCondition.Source.Lexeme != "assert" ||
		assertionCondition.Condition != assertions[0].Condition || withMessage.Refinement != assertionCondition {
		t.Fatalf("assertion condition fact = %+v, assertion = %+v", assertionCondition, withMessage)
	}
	proven, ok := analyzer.ResolvedAssertionOf(assertions[1])
	if !ok || proven.Reason != PanicReasonAssertionFailed || proven.HasMessage || proven.Message != "" || !proven.Proven {
		t.Fatalf("proven assertion fact = %+v", proven)
	}
	branchFunction := result.Program.Statements[2].(*ast.FunctionDeclaration)
	branch := branchFunction.Body.Statements[0].(*ast.IfStatement)
	branchCondition, ok := analyzer.ResolvedConditionFactOf(branch.Condition)
	if !ok || branchCondition.Kind != ConditionFactBranchTrue || branchCondition.Source.Lexeme != "if" || branchCondition.Condition != branch.Condition {
		t.Fatalf("branch condition fact = %+v", branchCondition)
	}
	before := len(analyzer.resolvedAssertions)
	if _, ok := analyzer.ResolvedAssertionOf(&ast.AssertStatement{}); ok || len(analyzer.resolvedAssertions) != before {
		t.Fatal("read-only assertion query resolved or mutated an unknown statement")
	}
}
