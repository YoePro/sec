package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestInvalidNewTargetTypeRetainsConstructionAndSiblings(t *testing.T) {
	input := `
fn Test() void {
	let broken := new Box[@](1)
	let preserved := 42
}

fn Next() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("expected invalid construction target to produce a parser error")
	}
	if len(program.Statements) != 2 {
		t.Fatalf("wrong declaration count. got=%d want=2", len(program.Statements))
	}

	testFn, ok := program.Statements[0].(*ast.FunctionDeclaration)
	if !ok || testFn.Body == nil || len(testFn.Body.Statements) != 2 {
		t.Fatalf("invalid construction lost its body or statement sibling: %#v", program.Statements[0])
	}
	broken := testFn.Body.Statements[0].(*ast.LetStatement)
	construction, ok := broken.Value.(*ast.NewExpression)
	if !ok {
		t.Fatalf("invalid construction was not retained as NewExpression. got=%T", broken.Value)
	}
	if construction.Type == nil || !construction.Type.Invalid || construction.Type.Recovery == nil || construction.Type.Name != "Box" {
		t.Fatalf("invalid construction target was not retained: %#v", construction.Type)
	}
	if len(construction.Type.TypeArgs) != 1 || !construction.Type.TypeArgs[0].Invalid || len(construction.Arguments) != 1 {
		t.Fatalf("partial target or parsed arguments were lost: %#v", construction)
	}

	if next, ok := program.Statements[1].(*ast.FunctionDeclaration); !ok || next.Name == nil || next.Name.Value != "Next" {
		t.Fatalf("following declaration was not retained: %#v", program.Statements[1])
	}
}
