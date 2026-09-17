package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestParseCheckedUnreachable verifies the canonical payload-free statement
// and its dedicated AST representation.
//
// Rules:
//   - rules/errors/panic.md — § 16(1) "Checked unreachable"
func TestParseCheckedUnreachable(t *testing.T) {
	path := "../../testdata/parser/checked_unreachable_valid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(source), path)).Parse()
	if result.HasErrors {
		t.Fatalf("unreachable diagnostics = %+v", result.Diagnostics)
	}
	function := result.Program.Statements[1].(*ast.FunctionDeclaration)
	statement, ok := function.Body.Statements[0].(*ast.UnreachableStatement)
	if !ok {
		t.Fatalf("statement = %T, want *ast.UnreachableStatement", function.Body.Statements[0])
	}
	if statement.Token.Lexeme != "unreachable" {
		t.Fatalf("token = %#v, want unreachable", statement.Token)
	}
	identifierFunction := result.Program.Statements[2].(*ast.FunctionDeclaration)
	if identifierFunction.Name.Value != "unreachable" {
		t.Fatalf("function name = %q, want contextual identifier unreachable", identifierFunction.Name.Value)
	}
	useFunction := result.Program.Statements[3].(*ast.FunctionDeclaration)
	returned := useFunction.Body.Statements[0].(*ast.ReturnStatement)
	call, ok := returned.Value.(*ast.CallExpression)
	if !ok {
		t.Fatalf("returned expression = %T, want *ast.CallExpression", returned.Value)
	}
	callee, ok := call.Callee.(*ast.Identifier)
	if !ok || callee.Value != "unreachable" {
		t.Fatalf("callee = %#v, want ordinary identifier unreachable", call.Callee)
	}
}
