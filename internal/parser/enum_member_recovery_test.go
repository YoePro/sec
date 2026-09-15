package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestInvalidEnumMembersRetainLaterMembersAndDeclarations(t *testing.T) {
	input := `
enum Broken int {
	@,
	Present
}

enum NextEnum int {
	Value
}

fn Next() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected malformed enum member to produce a parser error")
	}
	if len(program.Statements) != 3 {
		t.Fatalf("wrong declaration count. got=%d want=3", len(program.Statements))
	}

	broken, ok := program.Statements[0].(*ast.EnumDeclaration)
	if !ok || len(broken.Values) != 2 {
		t.Fatalf("malformed member lost the enum or later member: %#v", program.Statements[0])
	}
	invalid := broken.Values[0]
	if invalid == nil || !invalid.Invalid || invalid.Recovery == nil || invalid.Name != nil {
		t.Fatalf("malformed member was not retained as an invalid enum value: %#v", invalid)
	}
	if broken.Values[1].Name == nil || broken.Values[1].Name.Value != "Present" {
		t.Fatalf("later enum member was not retained: %#v", broken.Values[1])
	}

	if nextEnum, ok := program.Statements[1].(*ast.EnumDeclaration); !ok || nextEnum.Name == nil || nextEnum.Name.Value != "NextEnum" {
		t.Fatalf("following enum was not retained: %#v", program.Statements[1])
	}
	if next, ok := program.Statements[2].(*ast.FunctionDeclaration); !ok || next.Name == nil || next.Name.Value != "Next" {
		t.Fatalf("following function was not retained: %#v", program.Statements[2])
	}
}
