package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Comments are trivia at every match-arm boundary, including after a short
// arm body and immediately before the closing brace.
//
// Rules:
//   - rules/foundations/lexical_structure.md — §5 "Comments"
//   - rules/foundations/grammar.md — "Match expression"
func TestParseMatchArmBlockAcceptsCommentsBetweenArms(t *testing.T) {
	input := `fn Choose(value: Choice) int {
    return match value {
        // first arm
        Choice.Some(item) => item // trailing first arm
        // between arms
        Choice.None => 0
        // final comment
    }
}
`
	parser := New(lexer.New(input))
	program := parser.ParseProgram()
	checkParserErrors(t, parser)

	function, ok := program.Statements[0].(*ast.FunctionDeclaration)
	if !ok || function.Body == nil || len(function.Body.Statements) != 1 {
		t.Fatalf("unexpected parsed function: %#v", program.Statements)
	}
	returnStatement, ok := function.Body.Statements[0].(*ast.ReturnStatement)
	if !ok {
		t.Fatalf("function statement = %T, want ReturnStatement", function.Body.Statements[0])
	}
	match, ok := returnStatement.Value.(*ast.MatchExpression)
	if !ok {
		t.Fatalf("return value = %T, want MatchExpression", returnStatement.Value)
	}
	if len(match.Arms) != 2 {
		t.Fatalf("match arms = %d, want 2", len(match.Arms))
	}
	for index, arm := range match.Arms {
		if arm == nil || arm.Invalid {
			t.Fatalf("match arm %d was not retained as valid: %#v", index, arm)
		}
	}
}
