package parser

import (
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// matchArmFinalStatements parses a function whose first statement binds an
// expression match and returns the last statement of every block arm.
func matchArmFinalStatements(t *testing.T, input string) []ast.Statement {
	t.Helper()
	parser := New(lexer.New(input))
	program := parser.ParseProgram()
	checkParserErrors(t, parser)
	function, ok := program.Statements[0].(*ast.FunctionDeclaration)
	if !ok || function.Body == nil || len(function.Body.Statements) == 0 {
		t.Fatalf("unexpected parsed function: %#v", program.Statements)
	}
	declaration, ok := function.Body.Statements[0].(*ast.LetStatement)
	if !ok {
		t.Fatalf("first statement = %T, want LetStatement", function.Body.Statements[0])
	}
	match, ok := declaration.Value.(*ast.MatchExpression)
	if !ok {
		t.Fatalf("let value = %T, want MatchExpression", declaration.Value)
	}
	var finals []ast.Statement
	for _, arm := range match.Arms {
		if arm.BlockBody == nil || len(arm.BlockBody.Statements) == 0 {
			t.Fatalf("arm %s has no block statements", arm.Pattern.String())
		}
		finals = append(finals, arm.BlockBody.Statements[len(arm.BlockBody.Statements)-1])
	}
	return finals
}

// In expression-match result position a block arm ends with its contextual
// value expression. That expression may begin with a token that starts no
// statement: a move expression, a literal or a sign.
//
// Rules:
//   - rules/foundations/grammar.md — § 56(2) "Match arms"
//   - rules/foundations/grammar.md — § 87(1) "Move expression"
func TestParseMatchArmBlockFinalValueExpressions(t *testing.T) {
	finals := matchArmFinalStatements(t, `fn Pick(flag: Choice, box: Box) Box {
    let chosen := match flag {
        Choice.Move => {
            let local := box
            <-local
        }
        Choice.Literal => {
            42
        }
        Choice.Negative => {
            -1
        }
        Choice.Text => {
            "fallback"
        }
    }
    return chosen
}
`)
	want := []string{"<-local", "42", "-1", `"fallback"`}
	if len(finals) != len(want) {
		t.Fatalf("block arms = %d, want %d", len(finals), len(want))
	}
	for index, statement := range finals {
		expression, ok := statement.(*ast.ExpressionStatement)
		if !ok {
			t.Fatalf("arm %d final statement = %T, want ExpressionStatement", index, statement)
		}
		if got := strings.NewReplacer("(", "", ")", "", " ", "").Replace(expression.Expression.String()); got != want[index] {
			t.Errorf("arm %d final value = %q, want %q", index, got, want[index])
		}
	}
}

// A match statement that ends a value-producing arm block is that block's
// value, so its own block arms keep the result position.
//
// Rules:
//   - rules/foundations/grammar.md — § 56(2) "Match arms", § 57 "Match statement"
func TestParseNestedMatchKeepsArmResultPosition(t *testing.T) {
	finals := matchArmFinalStatements(t, `fn Pick(flag: Choice, inner: Choice) int {
    let chosen := match flag {
        Choice.Outer => {
            match inner {
                Choice.Inner => {
                    7
                }
                _ => {
                    0
                }
            }
        }
    }
    return chosen
}
`)
	nested, ok := finals[0].(*ast.MatchStatement)
	if !ok {
		t.Fatalf("final statement = %T, want MatchStatement", finals[0])
	}
	for _, arm := range nested.Match.Arms {
		if _, ok := arm.BlockBody.Statements[0].(*ast.ExpressionStatement); !ok {
			t.Fatalf("nested arm statement = %T, want ExpressionStatement", arm.BlockBody.Statements[0])
		}
	}
}

// The contextual value is not general block-expression syntax: an ordinary
// match statement arm, and a value that is not the last thing in the block,
// keep the unexpected-token diagnostic.
//
// Rules:
//   - rules/foundations/grammar.md — § 56(2): "This is not general block-expression syntax."
func TestParseMatchArmBlockFinalValueStaysContextual(t *testing.T) {
	tests := map[string]string{
		"match statement arm": `fn Run(flag: Choice) void {
    match flag {
        Choice.Some => {
            42
        }
    }
}
`,
		"value before the last statement": `fn Run(flag: Choice) int {
    let chosen := match flag {
        Choice.Some => {
            42
            let later := 1
        }
    }
    return chosen
}
`,
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			parser := New(lexer.New(input))
			parser.ParseProgram()
			errors := parser.Errors()
			if len(errors) == 0 || !strings.Contains(errors[0], `unexpected token "42" at 4:13`) {
				t.Fatalf("errors = %v, want unexpected token \"42\" at 4:13", errors)
			}
		})
	}
}
