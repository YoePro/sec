package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TestInvalidMemberAccessRetainsReceiver verifies that a missing member name
// produces an explicit invalid postfix node instead of dropping the receiver
// or the enclosing return/function AST.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Member-access recovery"
//   - rules/compiler/parser_recovery.md — "Postfix expression"
//   - rules/compiler/parser_recovery.md — "Invalid expression"
func TestInvalidMemberAccessRetainsReceiver(t *testing.T) {
	result := New(lexer.NewWithFile("module main\nfn Value(value: int) int { return value.", "member.sec")).Parse()
	if !result.HasErrors {
		t.Fatal("missing member name must remain a parser error")
	}

	function := functionNamed(result.Program, "Value")
	if function == nil || function.Body == nil || len(function.Body.Statements) != 1 {
		t.Fatalf("lost enclosing function or return: %#v", function)
	}
	returned, ok := function.Body.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("body statement = %T, want return", function.Body.Statements[0])
	}
	invalid, ok := returned.Value.(*ast.InvalidExpression)
	if !ok {
		t.Fatalf("return value = %T, want InvalidExpression", returned.Value)
	}
	receiver, ok := invalid.Left.(*ast.Identifier)
	if !ok || receiver.Value != "value" || invalid.Operator.Type != lexer.DOT {
		t.Fatalf("invalid member = %#v, want retained value receiver and dot", invalid)
	}
	if invalid.Recovery == nil || invalid.Recovery.DiagnosticID != diagnostics.ParserInvalidExpression || invalid.Recovery.End.Type != lexer.EOF {
		t.Fatalf("invalid member recovery = %#v", invalid.Recovery)
	}
	if !hasParserDiagnostic(result.Diagnostics, diagnostics.ParserInvalidExpression, lexer.DOT) {
		t.Fatalf("diagnostics = %+v, want focused invalid-expression at dot", result.Diagnostics)
	}
}

// TestRepeatedDotMemberRecoveryDoesNotInventAccess verifies that `value. .name`
// stops at the malformed first access and still preserves following top-level
// declarations. The spaces deliberately keep the dots as two member tokens;
// contiguous `..` remains the lexer's range token.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Member-access recovery"
//   - rules/compiler/parser_recovery.md — "No silent reinterpretation"
func TestRepeatedDotMemberRecoveryDoesNotInventAccess(t *testing.T) {
	result := New(lexer.NewWithFile(`module main
fn Broken(value: int) int {
	return value. .other
}
fn Next() int { return 1 }
`, "member.sec")).Parse()
	if !result.HasErrors {
		t.Fatal("repeated dot must remain a parser error")
	}
	broken := functionNamed(result.Program, "Broken")
	if broken == nil || broken.Body == nil || len(broken.Body.Statements) == 0 {
		t.Fatalf("lost Broken function: %#v", broken)
	}
	returned, ok := broken.Body.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("Broken first statement = %T, want return", broken.Body.Statements[0])
	}
	invalid, ok := returned.Value.(*ast.InvalidExpression)
	if !ok || invalid.Left == nil || invalid.Operator.Type != lexer.DOT {
		t.Fatalf("return value = %#v, want invalid first member access", returned.Value)
	}
	if _, nested := invalid.Left.(*ast.MemberExpression); nested {
		t.Fatalf("repeated dot was silently reinterpreted: %#v", invalid)
	}
	if functionNamed(result.Program, "Next") == nil {
		t.Fatalf("following declaration was lost: %#v", result.Program.Statements)
	}
}

// TestContiguousDotsRemainRangeSyntax verifies that member recovery does not
// split the lexer's longest-match range token in `value..other` into two
// invented member-access operators.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Member-access recovery"
//   - rules/compiler/parser_recovery.md — "No silent reinterpretation"
func TestContiguousDotsRemainRangeSyntax(t *testing.T) {
	result := New(lexer.NewWithFile(`module main
fn Broken(value: int) int {
	return value..other
}
fn Next() int { return 1 }
`, "member.sec")).Parse()
	if !result.HasErrors {
		t.Fatal("range token in this expression context must remain a parser error")
	}
	broken := functionNamed(result.Program, "Broken")
	if broken == nil || broken.Body == nil || len(broken.Body.Statements) == 0 {
		t.Fatalf("lost Broken function: %#v", broken)
	}
	returned, ok := broken.Body.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("Broken first statement = %T, want return", broken.Body.Statements[0])
	}
	if _, member := returned.Value.(*ast.MemberExpression); member {
		t.Fatalf("range token was silently reinterpreted as member access: %#v", returned.Value)
	}
	if functionNamed(result.Program, "Next") == nil {
		t.Fatalf("following declaration was lost: %#v", result.Program.Statements)
	}
}

func hasParserDiagnostic(diagnosticsList []Diagnostic, id string, tokenType lexer.TokenType) bool {
	for _, diagnostic := range diagnosticsList {
		if diagnostic.ID == id && diagnostic.Primary.Type == tokenType {
			return true
		}
	}
	return false
}
