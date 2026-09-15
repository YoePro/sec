package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestMissingRegisterFieldWidthRetainsFieldsAndDeclarations(t *testing.T) {
	input := `
type Broken register[1] {
	Missing: bit[],
	Present: bit,
}

type NextRegister register[1] {
	Flag: bit,
}

fn Next() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected missing bit field width to produce a parser error")
	}
	if len(program.Statements) != 3 {
		t.Fatalf("wrong declaration count. got=%d want=3", len(program.Statements))
	}

	broken, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok || broken.RegisterType == nil || len(broken.RegisterType.Fields) != 2 {
		t.Fatalf("register declaration or field sibling was not retained: %#v", program.Statements[0])
	}
	missing := broken.RegisterType.Fields[0]
	invalidWidth, ok := missing.WidthExpression.(*ast.InvalidExpression)
	if !ok || invalidWidth.Recovery == nil || missing.Name.Value != "Missing" {
		t.Fatalf("missing field width was not retained as InvalidExpression: %#v", missing)
	}
	if broken.RegisterType.Fields[1].Name.Value != "Present" {
		t.Fatalf("following field was not retained: %#v", broken.RegisterType.Fields[1])
	}

	nextRegister, ok := program.Statements[1].(*ast.TypeDeclStatement)
	if !ok || nextRegister.RegisterType == nil || nextRegister.Name.Value != "NextRegister" {
		t.Fatalf("following register declaration was not retained: %#v", program.Statements[1])
	}
	if next, ok := program.Statements[2].(*ast.FunctionDeclaration); !ok || next.Name == nil || next.Name.Value != "Next" {
		t.Fatalf("following function was not retained: %#v", program.Statements[2])
	}
}
