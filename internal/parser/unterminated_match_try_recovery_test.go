package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Specialized expression blocks retain their completed children at EOF while
// remaining invalid ParseResults that cannot proceed to code generation.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Unterminated block diagnostics"
//   - rules/compiler/parser_recovery.md — "Other specialized unterminated constructs can still return nil"
func TestUnterminatedMatchAndTryBlocksRetainPartialChildren(t *testing.T) {
	for _, test := range []struct {
		name       string
		file       string
		diagnostic string
		assert     func(*testing.T, ast.Expression)
	}{
		{
			name:       "match",
			file:       "unterminated_match_invalid.sec",
			diagnostic: "unterminated match block",
			assert: func(t *testing.T, expression ast.Expression) {
				match, ok := expression.(*ast.MatchExpression)
				if !ok || len(match.Arms) != 2 {
					t.Fatalf("partial match = %#v, want two retained arms", expression)
				}
				if match.Arms[0].Body == nil || match.Arms[1].Body == nil {
					t.Fatalf("match arm bodies were discarded: %#v", match.Arms)
				}
			},
		},
		{
			name:       "try",
			file:       "unterminated_try_handlers_invalid.sec",
			diagnostic: "unterminated try handler block",
			assert: func(t *testing.T, expression ast.Expression) {
				try, ok := expression.(*ast.TryExpression)
				if !ok || len(try.Handlers) != 2 {
					t.Fatalf("partial try = %#v, want two retained handlers", expression)
				}
				if try.Handlers[0].Body == nil || try.Handlers[1].Body == nil {
					t.Fatalf("try handler bodies were discarded: %#v", try.Handlers)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			input, err := os.ReadFile("../../testdata/parser/" + test.file)
			if err != nil {
				t.Fatal(err)
			}
			result := New(lexer.New(string(input))).Parse()
			if !result.HasErrors || result.Fatal {
				t.Fatalf("expected recoverable parse error: %+v", result)
			}
			found := false
			for _, diagnostic := range result.Diagnostics {
				if strings.Contains(diagnostic.Message, test.diagnostic) && diagnostic.Primary.Type == lexer.EOF {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing %q at EOF: %+v", test.diagnostic, result.Diagnostics)
			}
			if len(result.Program.Statements) != 2 {
				t.Fatalf("lost enclosing declaration: %#v", result.Program.Statements)
			}
			function, ok := result.Program.Statements[1].(*ast.FunctionDeclaration)
			if !ok || function.Body == nil || len(function.Body.Statements) != 1 {
				t.Fatalf("lost partial function body: %#v", result.Program.Statements[1])
			}
			ret, ok := function.Body.Statements[0].(*ast.ReturnStatement)
			if !ok || ret.Value == nil {
				t.Fatalf("lost return expression: %#v", function.Body.Statements[0])
			}
			test.assert(t, ret.Value)
		})
	}
}
