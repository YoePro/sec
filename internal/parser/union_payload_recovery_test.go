package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Rules:
//   - rules/compiler/parser_recovery.md — "Union recovery"
//   - rules/compiler/parser_recovery.md — "Nil use"
//   - rules/declarations/unions.md — §3.2 "Single unnamed payload"
func TestInvalidUnionPayloadTypesRetainVariantsAndFollowingDeclarations(t *testing.T) {
	input := `
type Choice union {
    Empty()
    Malformed(@)
    MissingClose(string
    Later(uint)
}

fn Outside() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) != 3 {
		t.Fatalf("wrong parser error count. got=%d errors=%v", len(p.Errors()), p.Errors())
	}
	if len(program.Statements) != 2 {
		t.Fatalf("wrong declaration count. got=%d want=2", len(program.Statements))
	}

	declaration, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok || len(declaration.UnionVariants) != 4 {
		t.Fatalf("invalid payloads lost union variants: %#v", program.Statements[0])
	}
	for index := 0; index < 3; index++ {
		payload := declaration.UnionVariants[index].Payload
		if payload == nil || !payload.Invalid || payload.Recovery == nil {
			t.Fatalf("variant %d did not retain its invalid payload type: %#v", index, declaration.UnionVariants[index])
		}
	}
	if payload := declaration.UnionVariants[3].Payload; payload == nil || payload.Invalid || payload.Name != "uint" {
		t.Fatalf("following valid payload was not retained: %#v", declaration.UnionVariants[3])
	}
	if outside, ok := program.Statements[1].(*ast.FunctionDeclaration); !ok || outside.Name == nil || outside.Name.Value != "Outside" {
		t.Fatalf("following declaration was not retained: %#v", program.Statements[1])
	}
}
