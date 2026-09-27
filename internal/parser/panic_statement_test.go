package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// rules/errors/panic.md § 17 locks panic as a statement with an optional static
// string-literal payload.
func TestParsePanicStatement(t *testing.T) {
	path := "../../testdata/parser/panic_statements_valid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(source), path)).Parse()
	if result.HasErrors {
		t.Fatalf("panic diagnostics = %+v", result.Diagnostics)
	}
	function := result.Program.Statements[1].(*ast.FunctionDeclaration)
	statement, ok := function.Body.Statements[0].(*ast.PanicStatement)
	if !ok {
		t.Fatalf("statement = %T, want *ast.PanicStatement", function.Body.Statements[0])
	}
	if statement.Message == nil || statement.Message.Value != "explicit failure" {
		t.Fatalf("panic message = %#v, want explicit failure", statement.Message)
	}
	bareFunction := result.Program.Statements[2].(*ast.FunctionDeclaration)
	bare, ok := bareFunction.Body.Statements[0].(*ast.PanicStatement)
	if !ok {
		t.Fatalf("bare statement = %T, want *ast.PanicStatement", bareFunction.Body.Statements[0])
	}
	if bare.Message != nil {
		t.Fatalf("bare panic message = %#v, want nil", bare.Message)
	}
}

func TestRejectNonCanonicalPanicSpellings(t *testing.T) {
	path := "../../testdata/parser/panic_statements_invalid.sec"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parser := New(lexer.NewWithFile(string(source), path))
	parser.ParseProgram()
	errors := strings.Join(parser.Errors(), "\n")
	for _, want := range []string{
		"function-like panic(...) is not valid; write panic or panic \"message\"",
		"panic message, when present, must be a string literal",
	} {
		if !strings.Contains(errors, want) {
			t.Fatalf("errors = %q, want diagnostic containing %q", errors, want)
		}
	}
}
