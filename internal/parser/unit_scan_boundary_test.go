package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// BUG-000032: `a < b { ... c > (x) ... }` is a comparison followed by a
// block. The speculative unit-conversion scan stops at block and statement
// delimiters, so it never reaches the later `> (`; a real unit conversion on
// one expression still parses.
// Rules: rules/types/units.md — "Structural unit expressions"; rules/foundations/grammar.md — "Unit conversion expression".
func TestUnitScanStopsAtBlockBoundary(t *testing.T) {
	for _, source := range []string{
		"module main\n\nfn F(a: int, b: int, c: int) bool {\n    if a < b {\n        return c > (a)\n    }\n    return false\n}\n",
		"module main\n\nfn F(a: int, b: int, c: int) bool {\n    while a < c { if b > (a) { return true } }\n    return a < b || c > (b)\n}\n",
		"module main\n\nfn F(a: int, b: int) bool {\n    let x := a < b\n    let y := (b) > (a)\n    return x && y\n}\n",
	} {
		p := New(lexer.NewWithFile(source, "cmp.sec"))
		if result := p.Parse(); result.HasErrors {
			t.Fatalf("parse errors %v for\n%s", p.Errors(), source)
		}
	}
	p := New(lexer.NewWithFile("module main\n\nfn F(x: float<m>) float<km> {\n    return float<km>(x)\n}\n", "unit.sec"))
	result := p.Parse()
	if result.HasErrors {
		t.Fatalf("unit conversion: %v", p.Errors())
	}
	fn := result.Program.Statements[len(result.Program.Statements)-1].(*ast.FunctionDeclaration)
	ret := fn.Body.Statements[0].(*ast.ReturnStatement)
	if conversion, ok := ret.Value.(*ast.ConversionExpression); !ok || conversion.Type.Unit != "km" {
		t.Fatalf("unit conversion not retained: %#v", ret.Value)
	}
}
