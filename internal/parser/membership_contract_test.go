package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Empty membership syntax is retained as an AST contract so the semantic
// layer can issue types.empty-in-contract with type context and repair help.
//
// Rules:
//   - rules/foundations/grammar.md — "Type contracts", MembershipContract
//   - rules/types/contracts.md — "Ordered membership"
//   - rules/types/default_values.md — "Empty in [...] list"
func TestParseRetainsEmptyMembershipContractForSemanticDiagnostic(t *testing.T) {
	parser := New(lexer.New("type Impossible int in []\n"))
	program := parser.ParseProgram()
	checkParserErrors(t, parser)
	if len(program.Statements) != 1 {
		t.Fatalf("statements = %d, want 1", len(program.Statements))
	}
	declaration, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("statement = %T, want type declaration", program.Statements[0])
	}
	membership, ok := declaration.Contract.(*ast.MembershipContract)
	if !ok || len(membership.Values) != 0 {
		t.Fatalf("empty membership contract = %#v", declaration.Contract)
	}
}
