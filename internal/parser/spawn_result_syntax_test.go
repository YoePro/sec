package parser

import (
	"os"
	"strings"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Spawn recovery keeps a malformed value return distinct from void fallthrough
// and retains the source child boundary. Rules: rules/concurrency/spawn.md
// §§3A(3),25(5); rules/compiler/parser_recovery.md — Invalid AST nodes.
func TestSpawnResultRecovery(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/spawn_results/recovery_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range strings.Split(string(data), "// CASE ")[1:] {
		name, source, _ := strings.Cut(part, "\n")
		t.Run(name, func(t *testing.T) {
			p := New(lexer.New(source))
			program := p.ParseProgram()
			if len(p.Errors()) == 0 {
				t.Fatal("accepted invalid source")
			}
			spawns, returns := 0, 0
			ast.Visit(program, func(node any) bool {
				switch n := node.(type) {
				case *ast.SpawnExpression:
					spawns++
					if name != "return-value" {
						if _, ok := n.Value.(*ast.InvalidExpression); !ok {
							t.Fatalf("invalid operand lost: %T", n.Value)
						}
					}
				case *ast.ReturnStatement:
					returns++
					if _, ok := n.Value.(*ast.InvalidExpression); !ok {
						t.Fatalf("invalid returned value lost: %T", n.Value)
					}
				}
				return true
			})
			if spawns != 1 || (name == "return-value" && returns != 1) {
				t.Fatalf("lost source nodes: spawn=%d return=%d", spawns, returns)
			}
			last := program.Statements[len(program.Statements)-1]
			if f, ok := last.(*ast.FunctionDeclaration); !ok || f.Name.Value != "After" {
				t.Fatalf("lost sibling %T", last)
			}
		})
	}
}

// Explicit returns and discarded expression statements stay different source
// nodes through every spawn kind, including nested callable/control-flow bodies.
// Rules: rules/concurrency/spawn.md §§3A(1–4),25(5); grammar.md §89(4).
func TestSpawnResultSourceNodes(t *testing.T) {
	data, err := os.ReadFile("../../testdata/parser/spawn_results/forms.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	p := New(lexer.New(source))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	function := program.Statements[1].(*ast.FunctionDeclaration)
	for i, statement := range function.Body.Statements[:8] {
		spawn := statement.(*ast.LetStatement).Value.(*ast.SpawnExpression)
		if spawn.Body == nil || spawn.Value != nil {
			t.Fatalf("lost block %d", i)
		}
		if i%2 == 0 {
			if _, ok := spawn.Body.Statements[0].(*ast.ExpressionStatement); !ok {
				t.Fatalf("tail rewritten: %T", spawn.Body.Statements[0])
			}
		} else {
			if returned, ok := spawn.Body.Statements[0].(*ast.ReturnStatement); !ok || returned.Value == nil {
				t.Fatalf("return lost: %T", spawn.Body.Statements[0])
			}
		}
	}
	spawns, returns := 0, 0
	ast.Visit(program, func(node any) bool {
		switch n := node.(type) {
		case *ast.SpawnExpression:
			spawns++
			if n.EndToken.ByteEnd <= n.Token.ByteStart || n.EndToken.ByteEnd > len(source) {
				t.Fatalf("invalid endpoint: %+v", n.EndToken)
			}
			if n.Body != nil {
				if n.EndToken != n.Body.EndToken || n.EndToken.Type != lexer.RBRACE {
					t.Fatal("lost block endpoint")
				}
			} else if n.EndToken.Type != lexer.RPAREN {
				t.Fatalf("lost call endpoint: %+v", n.EndToken)
			}
			if source[n.Token.ByteStart:n.Token.ByteEnd] != "spawn" {
				t.Fatal("lost spawn span")
			}
			if n.KindToken.Type != "" && source[n.KindToken.ByteStart:n.KindToken.ByteEnd] != n.Kind {
				t.Fatal("lost kind span")
			}
		case *ast.ReturnStatement:
			returns++
		}
		return true
	})
	if spawns != 16 || returns != 10 {
		t.Fatalf("spawn=%d return=%d", spawns, returns)
	}
}
