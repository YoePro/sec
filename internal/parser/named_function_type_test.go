package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// A named type's base may be any TypeReference, including a function type
// whose `fn` keyword would otherwise start a following function declaration.
//
// Rules:
//   - rules/foundations/grammar.md — NamedTypeDeclaration and TypeReference (FunctionType)
//   - rules/types/types.md — "Function types"
func TestParseNamedFunctionTypeDeclaration(t *testing.T) {
	parser := New(lexer.New("type Callback fn(int, int) int\ntype Predicate -> fn(int) bool\nfn After() void {}\n"))
	program := parser.ParseProgram()
	checkParserErrors(t, parser)
	if len(program.Statements) != 3 {
		t.Fatalf("statements = %d, want 3", len(program.Statements))
	}
	callback := program.Statements[0].(*ast.TypeDeclStatement)
	if callback.BaseType == nil || len(callback.BaseType.FunctionParameterTypes) != 2 || callback.BaseType.FunctionReturnType == nil {
		t.Fatalf("Callback base = %#v, want fn(int, int) int", callback.BaseType)
	}
	if _, ok := program.Statements[2].(*ast.FunctionDeclaration); !ok {
		t.Fatalf("following statement = %T, want function declaration", program.Statements[2])
	}
}

// A type name followed by a function declaration on the next line still
// reports the missing base type instead of consuming that function.
//
// Rule: rules/compiler/parser_recovery.md — "Declaration-header recovery".
func TestTypeMissingBaseBeforeNextLineFunction(t *testing.T) {
	result := New(lexer.New("type Missing\nfn After() void {}\n")).Parse()
	if len(result.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v, want one missing-base-type diagnostic", result.Diagnostics)
	}
}
