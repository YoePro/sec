package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// `list[T] {}` and `list[T, Capacity] {}` are compiler-known collection
// literals, not struct literals, and the capacity constant is accepted in
// expression position.
//
// Rules:
//   - rules/collections/collections.md — §13.3 canonical explicit empty forms
//   - rules/types/default_values.md — "List defaults"
func TestParseEmptyListCollectionLiterals(t *testing.T) {
	parser := New(lexer.New("fn F() void {\n    let a := list[int] {}\n    let b := list[Packet, 32] {}\n}\n"))
	program := parser.ParseProgram()
	checkParserErrors(t, parser)
	body := program.Statements[0].(*ast.FunctionDeclaration).Body.Statements
	for index, wantCapacity := range []int{0, 1} {
		literal, ok := body[index].(*ast.LetStatement).Value.(*ast.CollectionLiteral)
		if !ok {
			t.Fatalf("statement %d value = %T, want *ast.CollectionLiteral", index, body[index].(*ast.LetStatement).Value)
		}
		if literal.Type.Name != "list" || len(literal.Type.TypeArgs) != 1 || len(literal.Type.ConstArgs) != wantCapacity || literal.Invalid {
			t.Fatalf("literal %d = %#v", index, literal.Type)
		}
	}
}

// List literal elements are not defined syntax: one diagnostic, the literal is
// retained as invalid, and the following statement survives.
//
// Rules:
//   - rules/collections/collections.md — §13.3
//   - rules/compiler/parser_recovery.md — "Bounded damage"
func TestParseNonEmptyListLiteralRecovers(t *testing.T) {
	result := New(lexer.New("fn F() void {\n    let a := list[int] {1, {2}, 3}\n    let b := 1\n}\n")).Parse()
	if len(result.Diagnostics) != 1 || !strings.Contains(result.Diagnostics[0].Message, "list literal must be empty") {
		t.Fatalf("diagnostics = %+v, want one empty-list diagnostic", result.Diagnostics)
	}
	body := result.Program.Statements[0].(*ast.FunctionDeclaration).Body.Statements
	if len(body) != 2 {
		t.Fatalf("body statements = %d, want 2", len(body))
	}
	if literal, ok := body[0].(*ast.LetStatement).Value.(*ast.CollectionLiteral); !ok || !literal.Invalid {
		t.Fatalf("value = %#v, want invalid collection literal", body[0].(*ast.LetStatement).Value)
	}
}
