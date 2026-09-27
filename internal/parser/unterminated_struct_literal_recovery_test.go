package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestUnterminatedStructLiteralRetainsFieldsAndEnclosingFunction verifies that
// EOF inserts the missing literal closer without discarding parsed fields or
// the enclosing return/function structure.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Struct literal recovery", "Missing closing brace"
//   - rules/compiler/parser_recovery.md — "Unterminated block diagnostics"
func TestUnterminatedStructLiteralRetainsFieldsAndEnclosingFunction(t *testing.T) {
	const fixture = "../../testdata/parser/unterminated_struct_literal_invalid.sec"
	input, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(input), fixture)).Parse()
	if !result.HasErrors {
		t.Fatal("unterminated struct literal must remain a parser error")
	}

	var function *ast.FunctionDeclaration
	for _, statement := range result.Program.Statements {
		if candidate, ok := statement.(*ast.FunctionDeclaration); ok && candidate.Name != nil && candidate.Name.Value == "Build" {
			function = candidate
			break
		}
	}
	if function == nil || function.Body == nil || len(function.Body.Statements) != 1 {
		t.Fatalf("lost enclosing Build function or return body: %#v", function)
	}
	returned, ok := function.Body.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("Build body statement = %T, want return", function.Body.Statements[0])
	}
	literal, ok := returned.Value.(*ast.StructLiteral)
	if !ok || literal.Type == nil || literal.Type.Name != "Point" {
		t.Fatalf("return value = %#v, want retained Point literal", returned.Value)
	}
	if len(literal.Fields) != 2 || literal.Fields[0].Name.Value != "x" || literal.Fields[1].Name.Value != "y" {
		t.Fatalf("retained fields = %#v, want x and y", literal.Fields)
	}

	foundCloser := false
	for _, event := range result.Recovery {
		if event.Kind == RecoveryInsertMissingToken && len(event.Expected) == 1 && event.Expected[0] == lexer.RBRACE && event.Start.Type == lexer.EOF {
			foundCloser = true
			break
		}
	}
	if !foundCloser {
		t.Fatalf("missing virtual struct-literal closer event: %+v", result.Recovery)
	}
	for _, diagnostic := range result.Diagnostics {
		if strings.Contains(diagnostic.Message, "expected ',' or '}' after struct literal field") {
			t.Fatalf("EOF was misdiagnosed as an invalid field separator: %+v", result.Diagnostics)
		}
	}
}
