package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Rules:
//   - rules/declarations/properties.md — §2 "Property forms", §16 "Diagnostics"
//   - rules/compiler/parser_recovery.md — "Property recovery"
//   - rules/compiler/parser_recovery.md — "Nil use"
func TestMissingImplPropertyTypeRetainsPropertyAccessorsAndMembers(t *testing.T) {
	input := `
impl Record {
    property Value: {
        get { return 0 }
    }

    fn Next() void {}
}

fn Outside() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d errors=%v", len(p.Errors()), p.Errors())
	}
	if len(program.Statements) != 2 {
		t.Fatalf("wrong declaration count. got=%d want=2", len(program.Statements))
	}

	impl, ok := program.Statements[0].(*ast.ImplStatement)
	if !ok || len(impl.Members) != 2 {
		t.Fatalf("missing property type lost impl members: %#v", program.Statements[0])
	}
	property, ok := impl.Members[0].(*ast.PropertyDeclaration)
	if !ok || property.Type == nil || !property.Type.Invalid || property.Type.Recovery == nil || property.Getter == nil {
		t.Fatalf("property did not retain invalid type and getter: %#v", impl.Members[0])
	}
	method, ok := impl.Members[1].(*ast.FunctionDeclaration)
	if !ok || method.Name == nil || method.Name.Value != "Next" {
		t.Fatalf("following impl method was not retained: %#v", impl.Members[1])
	}
	if outside, ok := program.Statements[1].(*ast.FunctionDeclaration); !ok || outside.Name == nil || outside.Name.Value != "Outside" {
		t.Fatalf("following declaration was not retained: %#v", program.Statements[1])
	}
}
