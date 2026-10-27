package sema

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Array element ownership is explicit outside terminal return construction;
// duplicate moves and copying spreads never acquire executable transfer plans.
// Whole-array ownership does not authorize static or dynamic element move-out.
// Rules: rules/memory/copy_move.md — §§ 9.2, 10.1;
// rules/mlir/packages/sec-mlir-dialect_package17.md — §§ 15, 65.
func TestArrayLiteralOwnershipBoundaries(t *testing.T) {
	input, err := os.ReadFile("../../testdata/sema/owned_fixed_arrays_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(input)))
	program := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatal(p.Errors())
	}
	errors := NewAnalyzer().Analyze(program)
	for _, statement := range program.Statements {
		fn, ok := statement.(*ast.FunctionDeclaration)
		if !ok || fn.Name.Value == "Make" {
			continue
		}
		found := false
		for _, error := range errors {
			if error.Line > fn.Token.Line && error.Line < fn.Token.Line+5 {
				found = true
			}
		}
		if !found {
			t.Errorf("%s incorrectly accepted; errors: %v", fn.Name.Value, errors)
		}
	}
}
