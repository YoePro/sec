package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestMissingRegisterWidthRetainsRegisterFieldsAndDeclarations(t *testing.T) {
	input := `
type Broken register[] {
	Flag: bit
}

type Present register[1] {
	Flag: bit
}

fn Next() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected missing register width to produce a parser error")
	}
	if len(program.Statements) != 3 {
		t.Fatalf("wrong declaration count. got=%d want=3", len(program.Statements))
	}

	broken, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok || broken.Name == nil || broken.Name.Value != "Broken" || broken.RegisterType == nil {
		t.Fatalf("register declaration was not retained: %#v", program.Statements[0])
	}
	invalidWidth, ok := broken.RegisterType.WidthExpression.(*ast.InvalidExpression)
	if !ok || invalidWidth.Recovery == nil {
		t.Fatalf("missing width was not retained as InvalidExpression: %#v", broken.RegisterType.WidthExpression)
	}
	if len(broken.RegisterType.Fields) != 1 || broken.RegisterType.Fields[0].Name.Value != "Flag" {
		t.Fatalf("register fields were not retained: %#v", broken.RegisterType.Fields)
	}

	present, ok := program.Statements[1].(*ast.TypeDeclStatement)
	if !ok || present.RegisterType == nil || present.RegisterType.Width != 1 {
		t.Fatalf("following register declaration was not retained: %#v", program.Statements[1])
	}
	if next, ok := program.Statements[2].(*ast.FunctionDeclaration); !ok || next.Name == nil || next.Name.Value != "Next" {
		t.Fatalf("following function was not retained: %#v", program.Statements[2])
	}
}
