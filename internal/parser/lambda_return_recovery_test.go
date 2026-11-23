package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/ast/walk"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// A committed lambda before a real body retains its invalid return type,
// captures, parameters and body; later bindings/declarations stay outside it.
// Rules: rules/compiler/parser_recovery.md §§14.5,14.7,15,46.2;
// rules/declarations/lambda-functions.md §2.
func TestLambdaMissingReturnTypeRetainsBody(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/lambda_return_recovery/missing_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Diagnostics()) != 2 {
		t.Fatal(p.Errors())
	}
	for _, d := range p.Diagnostics() {
		if d.ID != diagnostics.ParserInvalidTypeReference {
			t.Fatal(d)
		}
	}
	lambdas := 0
	_ = walk.Inspect(program, func(node any) error {
		if l, ok := node.(*ast.LambdaExpression); ok {
			lambdas++
			if l.ReturnType == nil || !l.ReturnType.Invalid || l.ReturnType.Recovery == nil || l.Body == nil || len(l.Body.Statements) != 1 {
				t.Fatalf("lost lambda body/type: %+v", l)
			}
			if lambdas == 2 && len(l.Captures) != 1 {
				t.Fatal("lost captures")
			}
		}
		return nil
	})
	if lambdas != 2 || len(program.Statements) != 3 {
		t.Fatalf("lambdas=%d statements=%d", lambdas, len(program.Statements))
	}
	fn := program.Statements[1].(*ast.FunctionDeclaration)
	if len(fn.Body.Statements) != 3 || program.Statements[2].(*ast.FunctionDeclaration).Name.Value != "Later" {
		t.Fatal("lost later source")
	}
}
