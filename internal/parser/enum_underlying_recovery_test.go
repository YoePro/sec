package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestInvalidEnumUnderlyingTypesRetainEnumsAndValues(t *testing.T) {
	input := `
enum Missing: {
	First
}

enum Malformed: @ {
	Second
}

fn Next() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected invalid enum underlying types to produce parser errors")
	}
	if len(program.Statements) != 3 {
		t.Fatalf("wrong declaration count. got=%d want=3", len(program.Statements))
	}

	for index, name := range []string{"Missing", "Malformed"} {
		enum, ok := program.Statements[index].(*ast.EnumDeclaration)
		if !ok || enum.Name == nil || enum.Name.Value != name {
			t.Fatalf("enum %s was not retained: %#v", name, program.Statements[index])
		}
		if enum.UnderlyingType == nil || !enum.UnderlyingType.Invalid || enum.UnderlyingType.Recovery == nil {
			t.Fatalf("enum %s did not retain its invalid underlying type: %#v", name, enum.UnderlyingType)
		}
		if len(enum.Values) != 1 {
			t.Fatalf("enum %s lost its value: %#v", name, enum.Values)
		}
	}

	if next, ok := program.Statements[2].(*ast.FunctionDeclaration); !ok || next.Name == nil || next.Name.Value != "Next" {
		t.Fatalf("following declaration was not retained: %#v", program.Statements[2])
	}
}
