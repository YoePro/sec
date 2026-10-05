package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// The Shape[Rank] argument is type metadata, not a static tensor extent
// expression. The parser retains that distinction for Sema.
//
// Rules:
//   - rules/collections/shaped-types.md — §3.4 "Runtime-shaped owning tensor"
func TestParseRuntimeShapedOwningTensorType(t *testing.T) {
	p := New(lexer.New("fn Use(value: tensor[float32, Shape[3]]) void { return }"))
	program := p.ParseProgram()
	checkParserErrors(t, p)

	fn := program.Statements[0].(*ast.FunctionDeclaration)
	typ := fn.Parameters[0].Type
	if typ.Name != "tensor" || len(typ.TypeArgs) != 2 || len(typ.ConstArgs) != 0 {
		t.Fatalf("tensor type = %#v, want two type arguments and no static extents", typ)
	}
	shape := typ.TypeArgs[1]
	if shape.Name != "Shape" || len(shape.TypeArgs) != 0 || len(shape.ConstArgs) != 1 || shape.ConstArgs[0].String() != "3" {
		t.Fatalf("runtime shape argument = %#v, want Shape[3]", shape)
	}
}
