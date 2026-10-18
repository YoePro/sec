package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// A one-line block comment with code before and after it on the same line is
// inline trivia: a struct field, an infix expression and a parameter list
// parse as if it were absent, and it is still preserved as an inline comment
// attachment.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §§5.4–5.5
//   - rules/tooling/formatter.md — § 12(9a)
func TestParseInlineBlockCommentsAsTrivia(t *testing.T) {
	input := `type Example struct {
    c: int /* inline */,
    d: int,
}

fn F(a: int /* x */, b: int) int {
    let v := a /* y */ + b
    return v
}
`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	declaration, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok {
		t.Fatalf("first statement %T", program.Statements[0])
	}
	if declaration.StructType == nil || len(declaration.StructType.Fields) != 2 {
		t.Fatalf("struct fields: %+v", declaration.StructType)
	}
	function, ok := program.Statements[1].(*ast.FunctionDeclaration)
	if !ok || len(function.Parameters) != 2 {
		t.Fatalf("function parameters: %+v", program.Statements[1])
	}
	let, ok := function.Body.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("let statement %T", function.Body.Statements[0])
	}
	if infix, ok := let.Value.(*ast.InfixExpression); !ok || infix.Operator != "+" {
		t.Fatalf("inline comment split the expression: %T %v", let.Value, let.Value)
	}
	inline := 0
	for _, attachment := range program.Comments {
		if attachment.Placement == ast.CommentInline {
			inline++
		}
	}
	if inline != 3 {
		t.Fatalf("want 3 inline comment attachments, got %d: %+v", inline, program.Comments)
	}
}
