package sema

import (
	"os"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Invalid lambda signatures do not become void callables or suppress independent
// errors in later source. Their parser-owned invalid type yields no duplicate.
// Rules: rules/compiler/parser_recovery.md §§14.7,15,46.2,82.2.
func TestLambdaReturnRecoverySema(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/lambda_return_recovery/missing_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(data)))
	program := p.ParseProgram()
	if len(p.Errors()) != 2 {
		t.Fatal(p.Errors())
	}
	a := NewAnalyzer()
	errors := a.Analyze(program)
	if len(errors) != 2 {
		t.Fatal(errors)
	}
	if errors[0].Line != 5 || errors[1].Line != 9 {
		t.Fatal("independent source errors lost", errors)
	}
	if len(a.resolvedCallableIdentities) != 0 {
		t.Fatal("invalid lambda acquired a valid callable identity")
	}
}
