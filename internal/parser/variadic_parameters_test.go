package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestParseNativeVariadicFunctionParameter(t *testing.T) {
	parser := New(lexer.New(`
fn Sum(numbers: ...int) int {
	return 0
}
`))
	program := parser.ParseProgram()
	errors := parser.Errors()
	if len(errors) != 0 {
		t.Fatalf("parser errors = %v", errors)
	}

	function, ok := program.Statements[0].(*ast.FunctionDeclaration)
	if !ok {
		t.Fatalf("statement = %T, want function declaration", program.Statements[0])
	}
	if len(function.Parameters) != 1 || !function.Parameters[0].Variadic {
		t.Fatalf("parameters = %+v, want one native variadic parameter", function.Parameters)
	}
	if got := function.Parameters[0].Type.Name; got != "int" {
		t.Fatalf("variadic element type = %q, want int", got)
	}
}

func TestParseRejectsNonFinalNativeVariadicParameter(t *testing.T) {
	parser := New(lexer.New(`fn Invalid(values: ...int, suffix: int) void {}`))
	parser.ParseProgram()
	errors := parser.Errors()
	if len(errors) == 0 || !strings.Contains(errors[0], "variadic parameter must be last") {
		t.Fatalf("parser errors = %v, want final-parameter diagnostic", errors)
	}
}
