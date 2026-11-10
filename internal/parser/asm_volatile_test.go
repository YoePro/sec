package parser

import (
	"os"
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
	"strings"
	"testing"
)

// TestAsmVolatileSyntax rejects C-style syntax while preserving ordinary asm,
// volatile identifiers and the syntax after each rejected statement.
// Rules: rules/platform/inline_assembly.md — §46.
func TestAsmVolatileSyntax(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		name := "asm_volatile_identifier_valid.sec"
		want := 0
		if invalid {
			name = "asm_volatile_invalid.sec"
			want = 3
		}
		data, err := os.ReadFile("../../testdata/parser/" + name)
		if err != nil {
			t.Fatal(err)
		}
		p := New(lexer.New(string(data)))
		program := p.ParseProgram()
		if len(p.Errors()) != want {
			t.Fatal(name, p.Errors())
		}
		for _, d := range p.Diagnostics() {
			if d.ID != diagnostics.ParserReservedSyntax || !strings.Contains(d.Message, "use ordinary asm") {
				t.Fatal(d)
			}
		}
		if invalid {
			if len(program.Statements) != 3 {
				t.Fatal(program.Statements)
			}
			fn := program.Statements[1].(*ast.FunctionDeclaration)
			block := fn.Body.Statements[0].(*ast.UnsafeStatement).Body
			if len(block.Statements) != 6 {
				t.Fatal(block.Statements)
			}
			for i, stmt := range block.Statements {
				if i%2 == 0 {
					bad, ok := stmt.(*ast.InvalidStatement)
					if !ok || bad.Recovery == nil || !bad.Recovery.Reported {
						t.Fatalf("%T", stmt)
					}
				} else if _, ok := stmt.(*ast.LetStatement); !ok {
					t.Fatalf("%T", stmt)
				}
			}
		}
	}
}
