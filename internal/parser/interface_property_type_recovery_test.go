package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestInvalidInterfacePropertyTypesRetainPropertiesAndMembers(t *testing.T) {
	input := `
interface Service {
	property Missing: {
		get
	}
	property Malformed: @ {
		get
	}
	fn Next() void
}

fn Outside() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected invalid interface property types to produce parser errors")
	}
	if len(program.Statements) != 2 {
		t.Fatalf("wrong declaration count. got=%d want=2", len(program.Statements))
	}

	iface, ok := program.Statements[0].(*ast.InterfaceDeclaration)
	if !ok || len(iface.Properties) != 2 || len(iface.Methods) != 1 {
		t.Fatalf("invalid property types lost interface members: %#v", program.Statements[0])
	}
	for index, property := range iface.Properties {
		if property.Type == nil || !property.Type.Invalid || property.Type.Recovery == nil || !property.RequiresGet {
			t.Fatalf("property %d did not retain its invalid type and getter: %#v", index, property)
		}
	}
	if iface.Methods[0].Name.Value != "Next" {
		t.Fatalf("following interface method was not retained: %#v", iface.Methods[0])
	}
	if outside, ok := program.Statements[1].(*ast.FunctionDeclaration); !ok || outside.Name == nil || outside.Name.Value != "Outside" {
		t.Fatalf("following declaration was not retained: %#v", program.Statements[1])
	}
}
