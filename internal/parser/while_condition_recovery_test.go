package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TestMissingWhileConditionRetainsBodyAndFollowingSyntax verifies that a
// missing condition becomes explicit invalid syntax without losing the loop
// body, the next statement in the function, or the next declaration.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "While recovery", "Missing condition"
//   - rules/compiler/parser_recovery.md — "Recovery goals"
func TestMissingWhileConditionRetainsBodyAndFollowingSyntax(t *testing.T) {
	result := New(lexer.NewWithFile(`module main
fn Broken() void {
	while {
		let inside := 1
		break
	}
	let later := 2
}
fn Next() int { return 3 }
`, "while.sec")).Parse()
	if !result.HasErrors {
		t.Fatal("missing while condition must remain a parser error")
	}

	broken := functionNamed(result.Program, "Broken")
	if broken == nil || broken.Body == nil || len(broken.Body.Statements) != 2 {
		t.Fatalf("Broken body was not retained: %#v", broken)
	}
	loop, ok := broken.Body.Statements[0].(*ast.WhileStatement)
	if !ok {
		t.Fatalf("first statement = %T, want WhileStatement", broken.Body.Statements[0])
	}
	invalid, ok := loop.Condition.(*ast.InvalidExpression)
	if !ok || invalid.Recovery == nil || invalid.Recovery.DiagnosticID != diagnostics.ParserInvalidExpression || invalid.Token.Type != lexer.LBRACE {
		t.Fatalf("condition = %#v, want brace-anchored InvalidExpression", loop.Condition)
	}
	if loop.Body == nil || len(loop.Body.Statements) != 2 {
		t.Fatalf("while body was not retained: %#v", loop.Body)
	}
	if _, ok := broken.Body.Statements[1].(*ast.LetStatement); !ok {
		t.Fatalf("statement after recovered while = %T, want LetStatement", broken.Body.Statements[1])
	}
	if functionNamed(result.Program, "Next") == nil {
		t.Fatalf("following declaration was lost: %#v", result.Program.Statements)
	}
	if countParserDiagnostics(result.Diagnostics, diagnostics.ParserInvalidExpression, lexer.LBRACE) != 1 {
		t.Fatalf("diagnostics = %+v, want one focused missing-condition diagnostic", result.Diagnostics)
	}
}
