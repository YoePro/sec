package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Array literal elements and call arguments may be `lower..upper` or
// `lower..<upper` range segments; a missing upper bound is a focused error
// that keeps the literal.
//
// Rules:
//   - rules/collections/collections.md — § 5.6a "Range segments in array literals", § 6.7 "Append"
//   - rules/foundations/grammar.md — "Array literal"
func TestRangeSegmentsParseInArrayLiteralsAndArguments(t *testing.T) {
	p := New(lexer.New(`module main

fn F() void {
    let r := [224r..246r, 7, 0..<3]
    try values.Append(0r..127r)
}
`))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	body := program.Statements[1].(*ast.FunctionDeclaration).Body.Statements
	literal := body[0].(*ast.LetStatement).Value.(*ast.ArrayLiteral)
	if len(literal.Elements) != 3 {
		t.Fatalf("elements = %d", len(literal.Elements))
	}
	first, ok := literal.Elements[0].(*ast.RangeExpression)
	if !ok || first.Exclusive || first.Start.String() != "224r" || first.End.String() != "246r" {
		t.Fatalf("first segment = %#v", literal.Elements[0])
	}
	if last, ok := literal.Elements[2].(*ast.RangeExpression); !ok || !last.Exclusive {
		t.Fatalf("last segment = %#v", literal.Elements[2])
	}
	call := body[1].(*ast.ExpressionStatement).Expression.(*ast.TryExpression).Expression.(*ast.CallExpression)
	if segment, ok := call.Arguments[0].(*ast.RangeExpression); !ok || segment.Start.String() != "0r" || segment.End.String() != "127r" {
		t.Fatalf("Append argument = %#v", call.Arguments[0])
	}

	p = New(lexer.New("module main\n\nfn F() void {\n    let r := [1.., 2]\n}\n"))
	p.ParseProgram()
	if errors := p.Errors(); len(errors) == 0 || !strings.Contains(errors[0], "range segment requires an upper bound at 4:18") {
		t.Fatalf("errors = %v", errors)
	}
}
