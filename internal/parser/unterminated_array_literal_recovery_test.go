package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestUnterminatedArrayLiteralRetainsElementsAndEnclosingFunction verifies
// that EOF inserts the missing bracket without discarding completed elements
// or the surrounding return/function AST.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Array-literal recovery"
//   - rules/compiler/parser_recovery.md — "Recovery goals"
func TestUnterminatedArrayLiteralRetainsElementsAndEnclosingFunction(t *testing.T) {
	const fixture = "../../testdata/parser/unterminated_array_literal_invalid.sec"
	input, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(input), fixture)).Parse()
	if !result.HasErrors {
		t.Fatal("unterminated array literal must remain a parser error")
	}

	var function *ast.FunctionDeclaration
	for _, statement := range result.Program.Statements {
		if candidate, ok := statement.(*ast.FunctionDeclaration); ok && candidate.Name != nil && candidate.Name.Value == "Values" {
			function = candidate
			break
		}
	}
	if function == nil || function.Body == nil || len(function.Body.Statements) != 1 {
		t.Fatalf("lost enclosing Values function or return body: %#v", function)
	}
	returned, ok := function.Body.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("Values body statement = %T, want return", function.Body.Statements[0])
	}
	literal, ok := returned.Value.(*ast.ArrayLiteral)
	if !ok || len(literal.Elements) != 2 {
		t.Fatalf("return value = %#v, want retained two-element array literal", returned.Value)
	}

	foundCloser := false
	for _, event := range result.Recovery {
		if event.Kind == RecoveryInsertMissingToken && len(event.Expected) == 1 && event.Expected[0] == lexer.RBRACKET && event.Start.Type == lexer.EOF {
			foundCloser = true
			break
		}
	}
	if !foundCloser {
		t.Fatalf("missing virtual array-literal closer event: %+v", result.Recovery)
	}
	for _, diagnostic := range result.Diagnostics {
		if strings.Contains(diagnostic.Message, "expected ',' or ']' after array literal element") {
			t.Fatalf("EOF was misdiagnosed as an invalid element separator: %+v", result.Diagnostics)
		}
	}
}
