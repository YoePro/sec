package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Unterminated postfix brackets retain their base and parsed bounds at EOF,
// while the virtual closer remains a parser error that blocks compilation.
//
// Rules:
//   - rules/compiler/parser_recovery.md — "Index and slice recovery"
//   - rules/compiler/parser_recovery.md — "Postfix expression"
//   - rules/compiler/parser_recovery.md — "Recovery goals"
func TestUnterminatedIndexAndSliceRetainPartialExpressions(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		check   func(*testing.T, ast.Expression)
	}{
		{
			name:    "index",
			fixture: "../../testdata/parser/unterminated_index_invalid.sec",
			check: func(t *testing.T, expression ast.Expression) {
				index, ok := expression.(*ast.IndexExpression)
				if !ok || index.Left.String() != "values" || index.Index.String() != "1" {
					t.Fatalf("return value = %#v, want retained values[1] index", expression)
				}
			},
		},
		{
			name:    "slice",
			fixture: "../../testdata/parser/unterminated_slice_invalid.sec",
			check: func(t *testing.T, expression ast.Expression) {
				slice, ok := expression.(*ast.SliceExpression)
				if !ok || slice.Left.String() != "values" || slice.Start.String() != "1" ||
					slice.End.String() != "3" || !slice.Exclusive {
					t.Fatalf("return value = %#v, want retained values[1..<3] slice", expression)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input, err := os.ReadFile(test.fixture)
			if err != nil {
				t.Fatal(err)
			}
			result := New(lexer.NewWithFile(string(input), test.fixture)).Parse()
			if !result.HasErrors {
				t.Fatal("unterminated postfix expression must remain a parser error")
			}

			function := functionNamed(result.Program, "Value")
			if function == nil || function.Body == nil || len(function.Body.Statements) != 1 {
				t.Fatalf("lost enclosing Value function or return body: %#v", function)
			}
			returned, ok := function.Body.Statements[0].(*ast.ReturnStatement)
			if !ok {
				t.Fatalf("Value body statement = %T, want return", function.Body.Statements[0])
			}
			test.check(t, returned.Value)
			assertVirtualRightBracketAtEOF(t, result.Recovery)
		})
	}
}

func TestUnterminatedEmptyIndexRetainsInvalidIndexPosition(t *testing.T) {
	result := New(lexer.New("module main\nfn Value(values: int[]) int { return values[")).Parse()
	if !result.HasErrors {
		t.Fatal("unterminated empty index must remain a parser error")
	}
	function := functionNamed(result.Program, "Value")
	if function == nil || function.Body == nil || len(function.Body.Statements) != 1 {
		t.Fatalf("lost enclosing Value function: %#v", function)
	}
	returned := function.Body.Statements[0].(*ast.ReturnStatement)
	index, ok := returned.Value.(*ast.IndexExpression)
	if !ok {
		t.Fatalf("return value = %T, want retained index", returned.Value)
	}
	if _, ok := index.Index.(*ast.InvalidExpression); !ok {
		t.Fatalf("index position = %#v, want InvalidExpression", index.Index)
	}
	assertVirtualRightBracketAtEOF(t, result.Recovery)
}

func functionNamed(program *ast.Program, name string) *ast.FunctionDeclaration {
	for _, statement := range program.Statements {
		if function, ok := statement.(*ast.FunctionDeclaration); ok && function.Name != nil && function.Name.Value == name {
			return function
		}
	}
	return nil
}

func assertVirtualRightBracketAtEOF(t *testing.T, recovery []RecoveryEvent) {
	t.Helper()
	for _, event := range recovery {
		if event.Kind == RecoveryInsertMissingToken && len(event.Expected) == 1 &&
			event.Expected[0] == lexer.RBRACKET && event.Start.Type == lexer.EOF {
			return
		}
	}
	t.Fatalf("missing virtual index/slice closer event: %+v", recovery)
}
