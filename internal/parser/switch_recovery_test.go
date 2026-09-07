package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

func TestUnterminatedSwitchRetainsClauses(t *testing.T) {
	for _, name := range []string{"unterminated_switch", "unterminated_empty_switch"} {
		t.Run(name, func(t *testing.T) {
			input, err := os.ReadFile("../../testdata/parser/" + name + "_invalid.sec")
			if err != nil {
				t.Fatal(err)
			}
			result := New(lexer.New(string(input))).Parse()
			if !result.HasErrors || result.Fatal {
				t.Fatalf("expected recoverable parse error: %+v", result)
			}
			found := false
			for _, d := range result.Diagnostics {
				if strings.Contains(d.Message, "unterminated switch body") && d.Primary.Type == lexer.EOF {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing switch diagnostic: %+v", result.Diagnostics)
			}
			if len(result.Program.Statements) != 2 {
				t.Fatalf("lost enclosing function: %+v", result.Program)
			}
			fn, ok := result.Program.Statements[1].(*ast.FunctionDeclaration)
			if !ok || fn.Body == nil || len(fn.Body.Statements) != 1 {
				t.Fatalf("lost function body: %#v", result.Program.Statements[1])
			}
			sw, ok := fn.Body.Statements[0].(*ast.SwitchStatement)
			if !ok {
				t.Fatalf("lost switch: %T", fn.Body.Statements[0])
			}
			if name == "unterminated_empty_switch" {
				if sw.Subject != nil || len(sw.Cases) != 0 || sw.Default != nil {
					t.Fatalf("fabricated switch contents: %+v", sw)
				}
				return
			}
			if len(sw.Cases) != 2 || sw.Default == nil {
				t.Fatalf("lost clauses: %+v", sw)
			}
			for i, clause := range append(sw.Cases, sw.Default) {
				if clause.Body == nil || len(clause.Body.Statements) != 1 {
					t.Fatalf("lost clause body: %+v", clause)
				}
				ret, ok := clause.Body.Statements[0].(*ast.ReturnStatement)
				if !ok {
					t.Fatalf("lost return: %T", clause.Body.Statements[0])
				}
				value, ok := ret.Value.(*ast.IntegerLiteral)
				if !ok || value.Value != int64((i+1)*10) {
					t.Fatalf("incorrect clause order/value: %#v", ret.Value)
				}
			}
		})
	}
}
