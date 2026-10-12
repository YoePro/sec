package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// `bit[N](value)` parses as a BitExtractionExpression, while an ordinary value
// named bit is still indexed and called normally (MD-046/MD-056).
//
// Rules:
//   - rules/foundations/grammar.md — BitExtractionExpression
func TestParseBitExtractionExpression(t *testing.T) {
	p := New(lexer.New("module main\n\nfn F(word: uint32, bit: int[4], i: int) void {\n    let low := bit[12](word + 1)\n    let element := bit[i]\n}\n"))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parse errors: %v", p.Errors())
	}
	function := program.Statements[len(program.Statements)-1].(*ast.FunctionDeclaration)
	extraction, ok := function.Body.Statements[0].(*ast.LetStatement).Value.(*ast.BitExtractionExpression)
	if !ok || extraction.Width.String() != "12" || extraction.Value.String() != "(word + 1)" || extraction.String() != "bit[12]((word + 1))" {
		t.Fatalf("extraction = %#v", function.Body.Statements[0].(*ast.LetStatement).Value)
	}
	if _, indexed := function.Body.Statements[1].(*ast.LetStatement).Value.(*ast.IndexExpression); !indexed {
		t.Fatalf("bit[i] without a call parsed as %T, want an index expression", function.Body.Statements[1].(*ast.LetStatement).Value)
	}
}
