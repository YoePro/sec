package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// A grouped expression reaching EOF retains its completed inner expression
// and enclosing return/function while recording a virtual closing parenthesis.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Grouped expression"
//   - rules/compiler/parser_recovery.md — "Recovery goals"
func TestUnterminatedGroupedExpressionRetainsInnerExpressionAndFunction(t *testing.T) {
	const fixture = "../../testdata/parser/unterminated_grouped_expression_invalid.sec"
	input, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(input), fixture)).Parse()
	if !result.HasErrors {
		t.Fatal("unterminated grouped expression must remain a parser error")
	}

	var function *ast.FunctionDeclaration
	for _, statement := range result.Program.Statements {
		if candidate, ok := statement.(*ast.FunctionDeclaration); ok && candidate.Name != nil && candidate.Name.Value == "Value" {
			function = candidate
			break
		}
	}
	if function == nil || function.Body == nil || len(function.Body.Statements) != 1 {
		t.Fatalf("lost enclosing Value function or return body: %#v", function)
	}
	returned, ok := function.Body.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("Value body statement = %T, want return", function.Body.Statements[0])
	}
	infix, ok := returned.Value.(*ast.InfixExpression)
	if !ok || infix.Operator != "+" {
		t.Fatalf("return value = %#v, want retained grouped addition", returned.Value)
	}

	foundCloser := false
	for _, event := range result.Recovery {
		if event.Kind == RecoveryInsertMissingToken && len(event.Expected) == 1 && event.Expected[0] == lexer.RPAREN && event.Start.Type == lexer.EOF {
			foundCloser = true
			break
		}
	}
	if !foundCloser {
		t.Fatalf("missing virtual grouped-expression closer event: %+v", result.Recovery)
	}
}
