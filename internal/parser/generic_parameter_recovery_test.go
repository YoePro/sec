package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestMalformedGenericParametersPreserveLaterParametersAndDeclarations(t *testing.T) {
	input := `
fn WithSibling[T: @, U](value: U) U {
	return value
}

fn MissingCloser[T, @(value: T) T {
	return value
}

fn Next() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected malformed generic parameters to produce parser errors")
	}
	if len(program.Statements) != 3 {
		t.Fatalf("wrong declaration count. got=%d want=3", len(program.Statements))
	}

	withSibling, ok := program.Statements[0].(*ast.FunctionDeclaration)
	if !ok || len(withSibling.GenericParameters) != 2 {
		t.Fatalf("malformed parameter lost its declaration or valid siblings: %#v", program.Statements[0])
	}
	if withSibling.GenericParameters[0].Name.Value != "T" || withSibling.GenericParameters[1].Name.Value != "U" {
		t.Fatalf("wrong recovered generic parameters: %#v", withSibling.GenericParameters)
	}
	if constraint := withSibling.GenericParameters[0].Constraint; constraint == nil || !constraint.Invalid || constraint.Recovery == nil {
		t.Fatalf("malformed constraint was not retained as an invalid type: %#v", constraint)
	}
	if withSibling.Body == nil || len(withSibling.Body.Statements) != 1 {
		t.Fatalf("recovered declaration lost its body: %#v", withSibling)
	}

	missingCloser, ok := program.Statements[1].(*ast.FunctionDeclaration)
	if !ok || len(missingCloser.GenericParameters) != 1 || missingCloser.GenericParameters[0].Name.Value != "T" {
		t.Fatalf("missing closer lost the declaration prefix: %#v", program.Statements[1])
	}
	if missingCloser.Body == nil || len(missingCloser.Body.Statements) != 1 {
		t.Fatalf("missing closer consumed the function boundary: %#v", missingCloser)
	}

	if next, ok := program.Statements[2].(*ast.FunctionDeclaration); !ok || next.Name == nil || next.Name.Value != "Next" {
		t.Fatalf("following declaration was not retained: %#v", program.Statements[2])
	}
}
