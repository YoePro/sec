package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Rules:
//   - rules/compiler/parser_recovery.md — "Struct declaration recovery"
//   - rules/compiler/parser_recovery.md — "Invalid field type"
func TestInvalidStructFieldTypesRetainFieldsAndFollowingDeclarations(t *testing.T) {
	input := `
type Record struct {
    Empty:,
    Malformed: @,
    Valid: int,
}

fn Outside() void {}
`

	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors()) != 2 {
		t.Fatalf("wrong parser error count. got=%d errors=%v", len(p.Errors()), p.Errors())
	}
	if len(program.Statements) != 2 {
		t.Fatalf("wrong declaration count. got=%d want=2", len(program.Statements))
	}

	declaration, ok := program.Statements[0].(*ast.TypeDeclStatement)
	if !ok || declaration.StructType == nil || len(declaration.StructType.Fields) != 3 {
		t.Fatalf("invalid types lost struct fields: %#v", program.Statements[0])
	}
	for index := 0; index < 2; index++ {
		field := declaration.StructType.Fields[index]
		if field.Type == nil || !field.Type.Invalid || field.Type.Recovery == nil {
			t.Fatalf("field %d did not retain its invalid type: %#v", index, field)
		}
	}
	valid := declaration.StructType.Fields[2]
	if valid.Name == nil || valid.Name.Value != "Valid" || valid.Type == nil || valid.Type.Invalid || valid.Type.Name != "int" {
		t.Fatalf("following valid field was not retained: %#v", valid)
	}
	if outside, ok := program.Statements[1].(*ast.FunctionDeclaration); !ok || outside.Name == nil || outside.Name.Value != "Outside" {
		t.Fatalf("following declaration was not retained: %#v", program.Statements[1])
	}
}
