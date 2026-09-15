package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

func TestInvalidExplicitGenericArgumentsRetainCallsAndSiblings(t *testing.T) {
	input := `
fn Test() void {
	let recovered := Identity[@, int](10)
	let empty := Identity[]()
	let preserved := 42
}

fn Next() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected malformed generic arguments to produce parser errors")
	}

	invalidTypeDiagnostics := 0
	for _, diagnostic := range p.Diagnostics() {
		if diagnostic.ID == diagnostics.ParserInvalidTypeReference {
			invalidTypeDiagnostics++
		}
	}
	if invalidTypeDiagnostics != 2 {
		t.Fatalf("wrong invalid type diagnostic count. got=%d want=2", invalidTypeDiagnostics)
	}

	if len(program.Statements) != 2 {
		t.Fatalf("wrong declaration count. got=%d want=2", len(program.Statements))
	}
	testFn, ok := program.Statements[0].(*ast.FunctionDeclaration)
	if !ok || testFn.Body == nil || len(testFn.Body.Statements) != 3 {
		t.Fatalf("malformed generic calls lost their enclosing body or siblings: %#v", program.Statements[0])
	}

	first := testFn.Body.Statements[0].(*ast.LetStatement)
	firstCall, ok := first.Value.(*ast.CallExpression)
	if !ok {
		t.Fatalf("recovered value is not CallExpression. got=%T", first.Value)
	}
	if len(firstCall.GenericArguments) != 2 || !firstCall.GenericArguments[0].Invalid || firstCall.GenericArguments[0].Recovery == nil || firstCall.GenericArguments[1].Name != "int" {
		t.Fatalf("generic argument siblings were not retained: %#v", firstCall.GenericArguments)
	}

	second := testFn.Body.Statements[1].(*ast.LetStatement)
	secondCall, ok := second.Value.(*ast.CallExpression)
	if !ok {
		t.Fatalf("empty-list value is not CallExpression. got=%T", second.Value)
	}
	if len(secondCall.GenericArguments) != 1 || !secondCall.GenericArguments[0].Invalid || secondCall.GenericArguments[0].Recovery == nil {
		t.Fatalf("empty generic list did not retain its invalid argument position: %#v", secondCall.GenericArguments)
	}

	if next, ok := program.Statements[1].(*ast.FunctionDeclaration); !ok || next.Name == nil || next.Name.Value != "Next" {
		t.Fatalf("following declaration was not retained: %#v", program.Statements[1])
	}
}
