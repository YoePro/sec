package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TestMissingCallArgumentRetainsPositionAndFollowingDeclarations verifies that
// an empty comma-delimited argument becomes an InvalidExpression without
// discarding later arguments, the call, or following declarations.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Argument-list recovery"
//   - rules/compiler/parser_recovery.md — "Progress"
func TestMissingCallArgumentRetainsPositionAndFollowingDeclarations(t *testing.T) {
	const fixture = "../../testdata/parser/missing_call_argument_invalid.sec"
	input, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	result := New(lexer.NewWithFile(string(input), fixture)).Parse()
	if !result.HasErrors {
		t.Fatal("missing call argument must remain a parser error")
	}

	var use *ast.FunctionDeclaration
	following := false
	for _, statement := range result.Program.Statements {
		function, ok := statement.(*ast.FunctionDeclaration)
		if !ok || function.Name == nil {
			continue
		}
		switch function.Name.Value {
		case "Use":
			use = function
		case "Following":
			following = true
		}
	}
	if use == nil || use.Body == nil || len(use.Body.Statements) != 1 || !following {
		t.Fatalf("lost Use or Following declaration: %#v", result.Program.Statements)
	}
	returned, ok := use.Body.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("Use body statement = %T, want return", use.Body.Statements[0])
	}
	call, ok := returned.Value.(*ast.CallExpression)
	if !ok || len(call.Arguments) != 3 {
		t.Fatalf("return value = %#v, want retained three-position call", returned.Value)
	}
	invalid, ok := call.Arguments[1].(*ast.InvalidExpression)
	if !ok || invalid.Recovery == nil || invalid.Recovery.DiagnosticID != diagnostics.ParserInvalidExpression || invalid.Token.Type != lexer.COMMA {
		t.Fatalf("middle argument = %#v, want comma-backed InvalidExpression", call.Arguments[1])
	}
	if integer, ok := call.Arguments[2].(*ast.IntegerLiteral); !ok || integer.Token.Lexeme != "30" {
		t.Fatalf("later argument = %#v, want retained 30", call.Arguments[2])
	}
}
