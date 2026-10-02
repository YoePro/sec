package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Range contract bounds are SignedNumericConstant. A same-line non-numeric
// bound gets one focused diagnostic, is consumed as an invalid expression, and
// never leaves stray module-scope code or swallows following declarations.
//
// Rules:
//   - rules/foundations/grammar.md — "Type contracts", RangeContract
//   - rules/compiler/parser_recovery.md — "Type-contract recovery", "Range contract"
func TestNonNumericRangeBoundsRecoverWithFocusedDiagnostic(t *testing.T) {
	source := "type A int range Max..100\ntype B int range 0..Max default 5\ntype C int range 0..(50 + 50)\nfn After() void {}\n"
	result := New(lexer.New(source)).Parse()
	if len(result.Diagnostics) != 3 {
		t.Fatalf("diagnostics = %+v, want 3", result.Diagnostics)
	}
	for index, want := range []string{"lower bound", "upper bound", "upper bound"} {
		if !strings.Contains(result.Diagnostics[index].Message, want) {
			t.Fatalf("diagnostic %d = %q, want %s", index, result.Diagnostics[index].Message, want)
		}
	}
	if len(result.Program.Statements) != 4 {
		t.Fatalf("statements = %d, want 4", len(result.Program.Statements))
	}
	lower := result.Program.Statements[0].(*ast.TypeDeclStatement).Contract.(*ast.RangeContract)
	if _, invalid := lower.Min.(*ast.InvalidExpression); !invalid {
		t.Fatalf("lower bound = %T, want invalid expression", lower.Min)
	}
	upper := result.Program.Statements[1].(*ast.TypeDeclStatement)
	if _, invalid := upper.Contract.(*ast.RangeContract).Max.(*ast.InvalidExpression); !invalid || upper.Default == nil {
		t.Fatalf("upper bound declaration = %#v, want invalid bound and retained default", upper)
	}
	if _, ok := result.Program.Statements[3].(*ast.FunctionDeclaration); !ok {
		t.Fatalf("following declaration = %T, want function", result.Program.Statements[3])
	}
}

// Open upper bounds at a line end, before a contract word, and before a
// default clause remain valid canonical syntax.
//
// Rule: rules/foundations/grammar.md — "Type contracts", RangeContract.
func TestOpenRangeBoundsRemainValid(t *testing.T) {
	parser := New(lexer.New("type A int range 1..\ntype B int range 0.. even\ntype C int range 1.. default 3\n"))
	parser.ParseProgram()
	checkParserErrors(t, parser)
}
