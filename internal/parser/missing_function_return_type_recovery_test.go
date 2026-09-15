package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestMissingFunctionReturnTypesRetainFunctionsAndSiblings(t *testing.T) {
	input := `
interface Service {
	fn Missing()
	fn Present() void
}

fn MissingBody() {
}

fn Next() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected missing return types to produce parser errors")
	}
	if len(program.Statements) != 3 {
		t.Fatalf("wrong declaration count. got=%d want=3", len(program.Statements))
	}

	iface, ok := program.Statements[0].(*ast.InterfaceDeclaration)
	if !ok || len(iface.Methods) != 2 {
		t.Fatalf("missing return type lost the interface or method sibling: %#v", program.Statements[0])
	}
	missingRequirement := iface.Methods[0]
	if missingRequirement.ReturnType == nil || !missingRequirement.ReturnType.Invalid || missingRequirement.ReturnType.Recovery == nil {
		t.Fatalf("interface requirement did not retain an invalid return type: %#v", missingRequirement)
	}
	if iface.Methods[1].Name.Value != "Present" || iface.Methods[1].ReturnType.Name != "void" {
		t.Fatalf("following interface method was not retained: %#v", iface.Methods[1])
	}

	missingBodyType, ok := program.Statements[1].(*ast.FunctionDeclaration)
	if !ok || missingBodyType.ReturnType == nil || !missingBodyType.ReturnType.Invalid || missingBodyType.ReturnType.Recovery == nil {
		t.Fatalf("ordinary function did not retain an invalid return type: %#v", program.Statements[1])
	}
	if missingBodyType.Body == nil {
		t.Fatalf("ordinary function body was not retained: %#v", missingBodyType)
	}

	if next, ok := program.Statements[2].(*ast.FunctionDeclaration); !ok || next.Name == nil || next.Name.Value != "Next" {
		t.Fatalf("following declaration was not retained: %#v", program.Statements[2])
	}
}
