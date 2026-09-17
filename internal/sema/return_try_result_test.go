package sema

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestReturnTryForwardsCompatibleResult verifies the narrow return-position
// rule without granting ordinary success values an implicit Result wrapping.
//
// Rules:
//   - rules/errors/errorhandling.md — §26 "return try expression"
//   - rules/errors/errorhandling.md — §37.8 "return try"
func TestReturnTryForwardsCompatibleResult(t *testing.T) {
	source := `module main
fn Source(value: int) Result[int, ArithmeticError] { return Ok(value) }
fn Forward(value: int) Result[int, ArithmeticError] { return try Source(value) }
`
	result := parser.New(lexer.New(source)).Parse()
	if result.HasErrors {
		t.Fatalf("parse: %v", result.Diagnostics)
	}
	a := NewAnalyzer()
	if errors := a.Analyze(result.Program); len(errors) != 0 {
		t.Fatalf("sema: %v", errors)
	}
	forward := result.Program.Statements[2].(*ast.FunctionDeclaration)
	tryExpr := forward.Body.Statements[0].(*ast.ReturnStatement).Value.(*ast.TryExpression)
	fact, ok := a.ResolvedTryOf(tryExpr)
	if !ok || fact.Kind != ResolvedTryResultReturnForwarding || fact.SuccessType.Kind != IntType || fact.EnclosingResultType.Kind != ResultType {
		t.Fatalf("resolved return try = %#v, %t", fact, ok)
	}
}

// TestReturnTryRejectsIncompatibleSuccessAndPlainSuccess checks both sides of
// the special-case boundary: only the success of a compatible Result operand
// is wrapped, never an arbitrary T returned from the function.
//
// Rules:
//   - rules/errors/errorhandling.md — §26 "return try expression"
//   - rules/errors/errorhandling.md — §37.8 "return try"
func TestReturnTryRejectsIncompatibleSuccessAndPlainSuccess(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main
fn Source() Result[int, ArithmeticError] { return Ok(1) }
fn WrongSuccess() Result[string, ArithmeticError] { return try Source() }
fn PlainSuccess() Result[int, ArithmeticError] { return 1 }
fn CheckedSuccess() Result[int, ArithmeticError] { return try 1 + 2 }
`)
	if len(errors) != 3 {
		t.Fatalf("errors = %#v", errors)
	}
	if !strings.Contains(errors[0].Message, "return try with success type int") || !strings.Contains(errors[0].Message, "requires Ok(string)") {
		t.Fatalf("wrong success diagnostic = %q", errors[0].Message)
	}
	if !strings.Contains(errors[1].Message, "must return Ok(...) or Err(...)") {
		t.Fatalf("plain success diagnostic = %q", errors[1].Message)
	}
	if !strings.Contains(errors[2].Message, "must return Ok(...) or Err(...)") {
		t.Fatalf("non-Result try diagnostic = %q", errors[2].Message)
	}
}

// TestReturnTryRejectsIncompatibleFailure retains the existing bodyless-try
// error compatibility check at the new forwarding boundary.
//
// Rules:
//   - rules/errors/errorhandling.md — §8 "try and error compatibility"
//   - rules/errors/errorhandling.md — §26 "return try expression"
func TestReturnTryRejectsIncompatibleFailure(t *testing.T) {
	errors := analyzeSourceRaw(t, `module main
enum StorageError error { Failed }
fn Source() Result[int, StorageError] { return Err(StorageError.Failed) }
fn Forward() Result[int, ArithmeticError] { return try Source() }
`)
	if len(errors) != 1 || !strings.Contains(errors[0].Message, "map StorageError to ArithmeticError") {
		t.Fatalf("errors = %#v", errors)
	}
}
