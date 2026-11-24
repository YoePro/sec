package formatter

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// Formatting is never a return-insertion fix. Retain every execution kind,
// callable/control-flow boundary, return and final expression (MD-133 T37).
// Rules: rules/concurrency/spawn.md §§3A(3),25(5); formatter.md §§5,17,25,29.
func TestSpawnResultRoundTrip(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/spawn_results/forms.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	before := spawnResultSourceShape(t, source, false)
	for _, indent := range []int{2, 4, 6} {
		options := Options{IndentationWidth: indent}
		result := Format(Source{Text: source}, options)
		if result.Malformed {
			t.Fatalf("malformed:\n%s", result.Text)
		}
		if !reflect.DeepEqual(before, spawnResultSourceShape(t, result.Text, false)) {
			t.Fatalf("spawn/return syntax changed:\n%s", result.Text)
		}
		if again := Format(Source{Text: result.Text}, options).Text; again != result.Text {
			t.Fatalf("not idempotent:\n%s", again)
		}
	}
}

// Malformed returned operands remain malformed value returns, never converted
// into void fallthrough during recovery/formatting.
// Rules: rules/concurrency/spawn.md §25(5); formatter.md §§25,29.
func TestSpawnResultRecoveryRoundTrip(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/spawn_results/recovery_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range strings.Split(string(data), "// CASE ")[1:] {
		name, source, _ := strings.Cut(part, "\n")
		t.Run(name, func(t *testing.T) {
			before := spawnResultSourceShape(t, source, true)
			result := Format(Source{Text: source}, Options{})
			if !reflect.DeepEqual(before, spawnResultSourceShape(t, result.Text, true)) {
				t.Fatalf("recovery shape changed:\n%s", result.Text)
			}
			if Format(Source{Text: result.Text}, Options{}).Text != result.Text {
				t.Fatal("not idempotent")
			}
		})
	}
}

// spawnResultSourceShape records source structure independently of whitespace
// and token positions. Rules: rules/concurrency/spawn.md §25(5); grammar.md §89(4).
func spawnResultSourceShape(t *testing.T, source string, invalid bool) []string {
	t.Helper()
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if (len(p.Errors()) > 0) != invalid {
		t.Fatalf("parse: %v\n%s", p.Errors(), source)
	}
	shape := []string{}
	ast.Visit(program, func(node any) bool {
		switch n := node.(type) {
		case *ast.SpawnExpression:
			shape = append(shape, fmt.Sprintf("spawn:%s:%s:block=%t", n.Kind, n.KindToken.Lexeme, n.Body != nil))
		case *ast.ReturnStatement:
			shape = append(shape, fmt.Sprintf("return:%T", n.Value))
		case *ast.ExpressionStatement:
			shape = append(shape, fmt.Sprintf("expression:%T", n.Expression))
		case *ast.BlockStatement:
			shape = append(shape, fmt.Sprintf("block:%d", len(n.Statements)))
		case *ast.IfStatement, *ast.WhileStatement, *ast.MatchExpression, *ast.DeferStatement, *ast.LambdaExpression, *ast.FunctionDeclaration:
			shape = append(shape, fmt.Sprintf("%T", n))
		}
		return true
	})
	return shape
}
