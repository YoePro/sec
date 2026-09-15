package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestInvalidImplementsEntriesRetainDeclarationsAndLaterInterfaces(t *testing.T) {
	input := `
interface Service implements @, Present {
	fn Run() void
}

impl Worker implements @, Present {
	fn Run() void {}
}

fn Outside() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected invalid implements entries to produce parser errors")
	}
	if len(program.Statements) != 3 {
		t.Fatalf("wrong declaration count. got=%d want=3", len(program.Statements))
	}

	iface, ok := program.Statements[0].(*ast.InterfaceDeclaration)
	if !ok || len(iface.Implements) != 2 || len(iface.Methods) != 1 {
		t.Fatalf("invalid conformance lost the interface or members: %#v", program.Statements[0])
	}
	assertRecoveredImplementsList(t, iface.Implements)

	impl, ok := program.Statements[1].(*ast.ImplStatement)
	if !ok || len(impl.Implements) != 2 || len(impl.Members) != 1 {
		t.Fatalf("invalid conformance lost the impl or members: %#v", program.Statements[1])
	}
	assertRecoveredImplementsList(t, impl.Implements)

	if outside, ok := program.Statements[2].(*ast.FunctionDeclaration); !ok || outside.Name == nil || outside.Name.Value != "Outside" {
		t.Fatalf("following declaration was not retained: %#v", program.Statements[2])
	}
}

func assertRecoveredImplementsList(t *testing.T, interfaces []*ast.TypeReference) {
	t.Helper()
	if interfaces[0] == nil || !interfaces[0].Invalid || interfaces[0].Recovery == nil {
		t.Fatalf("malformed interface position was not retained: %#v", interfaces[0])
	}
	if interfaces[1] == nil || interfaces[1].Name != "Present" || interfaces[1].Invalid {
		t.Fatalf("later valid interface was not retained: %#v", interfaces[1])
	}
}
