package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// TestMissingAssignmentValuesRetainTargetsAndBlockBoundaries verifies that
// ordinary and try assignments keep their target and operator, attach an
// invalid source expression, and leave the closing brace to the function body.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Assignment recovery"
//   - rules/compiler/parser_recovery.md — "Missing right expression"
//   - rules/compiler/parser_recovery.md — "Expression recovery"
func TestMissingAssignmentValuesRetainTargetsAndBlockBoundaries(t *testing.T) {
	result := New(lexer.NewWithFile(`module main
fn Plain() void {
	let mut value: int
	value =
}
fn Fallible() void {
	let mut value: int
	try value =
}
fn Next() int { return 1 }
`, "assignment.sec")).Parse()
	if !result.HasErrors {
		t.Fatal("missing assignment values must remain parser errors")
	}

	plain := functionNamed(result.Program, "Plain")
	if plain == nil || plain.Body == nil || len(plain.Body.Statements) != 2 {
		if plain != nil && plain.Body != nil {
			t.Fatalf("Plain statements = %d (%#v), diagnostics = %+v", len(plain.Body.Statements), plain.Body.Statements, result.Diagnostics)
		}
		t.Fatalf("lost Plain body: %#v; diagnostics = %+v", plain, result.Diagnostics)
	}
	assignment, ok := plain.Body.Statements[1].(*ast.AssignmentStatement)
	if !ok {
		t.Fatalf("Plain second statement = %T, want AssignmentStatement", plain.Body.Statements[1])
	}
	assertRecoveredAssignmentValue(t, assignment, "value", "=")

	fallible := functionNamed(result.Program, "Fallible")
	if fallible == nil || fallible.Body == nil || len(fallible.Body.Statements) != 2 {
		t.Fatalf("lost Fallible body: %#v", fallible)
	}
	tryAssignment, ok := fallible.Body.Statements[1].(*ast.TryAssignmentStatement)
	if !ok || tryAssignment.Assignment == nil {
		t.Fatalf("Fallible second statement = %#v, want TryAssignmentStatement", fallible.Body.Statements[1])
	}
	assertRecoveredAssignmentValue(t, tryAssignment.Assignment, "value", "=")

	if functionNamed(result.Program, "Next") == nil {
		t.Fatalf("following declaration was lost: %#v", result.Program.Statements)
	}
	if countParserDiagnostics(result.Diagnostics, diagnostics.ParserInvalidExpression, lexer.ASSIGN) != 2 {
		t.Fatalf("diagnostics = %+v, want two invalid assignment-value diagnostics", result.Diagnostics)
	}
}

func assertRecoveredAssignmentValue(t *testing.T, assignment *ast.AssignmentStatement, target string, operator string) {
	t.Helper()
	if assignment.Target == nil || assignment.Target.String() != target || assignment.Operator != operator {
		t.Fatalf("assignment = %#v, want retained %s %s", assignment, target, operator)
	}
	invalid, ok := assignment.Value.(*ast.InvalidExpression)
	if !ok || invalid.Recovery == nil || invalid.Recovery.End.Type != lexer.RBRACE {
		t.Fatalf("assignment value = %#v, want brace-bounded InvalidExpression", assignment.Value)
	}
}

func countParserDiagnostics(diagnosticsList []Diagnostic, id string, tokenType lexer.TokenType) int {
	count := 0
	for _, diagnostic := range diagnosticsList {
		if diagnostic.ID == id && diagnostic.Primary.Type == tokenType {
			count++
		}
	}
	return count
}
