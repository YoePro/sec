package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Range contract bounds are ordinary expressions evaluated by Sema in a
// SemanticCompileTimeRequiredContext (MD-011): named bindings, calls, and
// parenthesized arithmetic parse as bounds, expression parsing stops at the
// range operator, and following declarations and defaults survive.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 3.11–3.14, 3.40
//   - rules/foundations/grammar.md — "Type contracts", RangeContract
func TestRangeBoundsAreOrdinaryExpressions(t *testing.T) {
	source := "type A int range Max..100\ntype B int range 0..Max default 5\ntype C int range 0..(50 + 50)\ntype D int range Min..MaxPort()\nfn After() void {}\n"
	result := New(lexer.New(source)).Parse()
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v, want none", result.Diagnostics)
	}
	if len(result.Program.Statements) != 5 {
		t.Fatalf("statements = %d, want 5", len(result.Program.Statements))
	}
	lower := result.Program.Statements[0].(*ast.TypeDeclStatement).Contract.(*ast.RangeContract)
	if identifier, ok := lower.Min.(*ast.Identifier); !ok || identifier.Value != "Max" {
		t.Fatalf("lower bound = %#v, want identifier Max", lower.Min)
	}
	upper := result.Program.Statements[1].(*ast.TypeDeclStatement)
	if identifier, ok := upper.Contract.(*ast.RangeContract).Max.(*ast.Identifier); !ok || identifier.Value != "Max" || upper.Default == nil {
		t.Fatalf("upper bound declaration = %#v, want identifier bound and retained default", upper)
	}
	call := result.Program.Statements[3].(*ast.TypeDeclStatement).Contract.(*ast.RangeContract)
	if _, ok := call.Max.(*ast.CallExpression); !ok {
		t.Fatalf("call bound = %T, want call expression", call.Max)
	}
	if _, ok := result.Program.Statements[4].(*ast.FunctionDeclaration); !ok {
		t.Fatalf("following declaration = %T, want function", result.Program.Statements[4])
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
