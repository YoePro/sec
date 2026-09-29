package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TestMissingIfConditionsRetainBranchesAndFollowingDeclarations verifies that
// missing conditions become explicit invalid syntax nodes while consequence,
// else, nested else-if, later statements, and later declarations survive.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "If recovery", "Missing condition"
//   - rules/compiler/parser_recovery.md — "Recovery goals"
func TestMissingIfConditionsRetainBranchesAndFollowingDeclarations(t *testing.T) {
	result := New(lexer.NewWithFile(`module main
fn Broken(flag: bool) void {
	if {
		let inside := 1
	} else {
		let fallback := 2
	}
	if flag {
		let normal := 3
	} else if {
		let nested := 4
	}
	let later := 5
}
fn Next() int { return 6 }
`, "if.sec")).Parse()
	if !result.HasErrors {
		t.Fatal("missing if conditions must remain parser errors")
	}

	broken := functionNamed(result.Program, "Broken")
	if broken == nil || broken.Body == nil || len(broken.Body.Statements) != 3 {
		t.Fatalf("Broken body was not retained: %#v", broken)
	}
	first, ok := broken.Body.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("first statement = %T, want IfStatement", broken.Body.Statements[0])
	}
	assertRecoveredIfCondition(t, first)
	if first.Consequence == nil || len(first.Consequence.Statements) != 1 || first.Alternative == nil || len(first.Alternative.Statements) != 1 {
		t.Fatalf("first recovered branches = consequence %#v alternative %#v", first.Consequence, first.Alternative)
	}

	outer, ok := broken.Body.Statements[1].(*ast.IfStatement)
	if !ok || outer.Alternative == nil || len(outer.Alternative.Statements) != 1 {
		t.Fatalf("second statement = %#v, want if with else-if", broken.Body.Statements[1])
	}
	nested, ok := outer.Alternative.Statements[0].(*ast.IfStatement)
	if !ok {
		t.Fatalf("else branch statement = %T, want recovered IfStatement", outer.Alternative.Statements[0])
	}
	assertRecoveredIfCondition(t, nested)
	if nested.Consequence == nil || len(nested.Consequence.Statements) != 1 {
		t.Fatalf("nested consequence was lost: %#v", nested.Consequence)
	}

	if _, ok := broken.Body.Statements[2].(*ast.LetStatement); !ok {
		t.Fatalf("statement after recovered ifs = %T, want LetStatement", broken.Body.Statements[2])
	}
	if functionNamed(result.Program, "Next") == nil {
		t.Fatalf("following declaration was lost: %#v", result.Program.Statements)
	}
	if countParserDiagnostics(result.Diagnostics, diagnostics.ParserInvalidExpression, lexer.LBRACE) != 2 {
		t.Fatalf("diagnostics = %+v, want two focused missing-condition diagnostics", result.Diagnostics)
	}
}

func assertRecoveredIfCondition(t *testing.T, statement *ast.IfStatement) {
	t.Helper()
	invalid, ok := statement.Condition.(*ast.InvalidExpression)
	if !ok || invalid.Recovery == nil || invalid.Recovery.DiagnosticID != diagnostics.ParserInvalidExpression || invalid.Token.Type != lexer.LBRACE {
		t.Fatalf("condition = %#v, want brace-anchored InvalidExpression", statement.Condition)
	}
}
