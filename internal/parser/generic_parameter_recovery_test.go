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
	if constraints := withSibling.GenericParameters[0].Constraints; len(constraints) != 1 || !constraints[0].Invalid || constraints[0].Recovery == nil {
		t.Fatalf("malformed constraint was not retained as an invalid type: %#v", constraints)
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

func TestMissingConjoinedGenericConstraintPreservesFollowingParameter(t *testing.T) {
	input := `
fn Broken[T: First &, U](value: U) U {
	return value
}

fn Next() void {}
`
	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) != 1 || p.Errors()[0] != "expected constraint type after '&' for generic parameter T at 2:21" {
		t.Fatalf("constraint recovery errors = %v", p.Errors())
	}
	if len(program.Statements) != 2 {
		t.Fatalf("wrong declaration count: %d", len(program.Statements))
	}
	broken := program.Statements[0].(*ast.FunctionDeclaration)
	if len(broken.GenericParameters) != 2 || broken.GenericParameters[1].Name.Value != "U" {
		t.Fatalf("later generic parameter was not preserved: %+v", broken.GenericParameters)
	}
	constraints := broken.GenericParameters[0].Constraints
	if len(constraints) != 2 || constraints[0].Name != "First" || !constraints[1].Invalid {
		t.Fatalf("partial constraint set was not preserved: %+v", constraints)
	}
}
