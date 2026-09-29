package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Rules:
//   - rules/declarations/properties.md — §3 "Setter parameter is always explicit"
//   - rules/compiler/parser_recovery.md — "Property recovery", "Missing setter parameter"
func TestMissingSetterParameterRetainsSetterBodyAndLaterGetter(t *testing.T) {
	input := `
impl Record {
    property Value: int {
        set {
            discard 1
        }

        get {
            return 0
        }
    }

    fn Next() void {}
}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) != 1 {
		t.Fatalf("wrong parser error count. got=%d errors=%v", len(p.Errors()), p.Errors())
	}

	impl := program.Statements[0].(*ast.ImplStatement)
	if len(impl.Members) != 2 {
		t.Fatalf("invalid setter lost impl members: %#v", impl.Members)
	}
	property, ok := impl.Members[0].(*ast.PropertyDeclaration)
	if !ok || property.Setter == nil || !property.Setter.Invalid || property.Setter.Recovery == nil || property.Setter.Parameter != nil || property.Setter.Body == nil {
		t.Fatalf("missing parameter did not retain an invalid setter and body: %#v", impl.Members[0])
	}
	if property.Getter == nil {
		t.Fatalf("invalid setter discarded later getter: %#v", property)
	}
	method, ok := impl.Members[1].(*ast.FunctionDeclaration)
	if !ok || method.Name == nil || method.Name.Value != "Next" {
		t.Fatalf("invalid setter discarded later impl method: %#v", impl.Members[1])
	}
}
