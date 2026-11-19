package cst

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
	"sec/internal/parser"
)

// TestTryHandlerListBlockRoles identifies actual handler lists independently of
// operand/configuration literals, error field patterns and bodyless try forms.
// Rules: rules/tooling/formatter.md — §§5,8,19(2–6);
// rules/errors/errorhandling.md — §§15,23.
func TestTryHandlerListBlockRoles(t *testing.T) {
	data, err := os.ReadFile("../../testdata/formatter/try_handler_blocks.sec")
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	document := Build(source, "")
	document.ApplyProgramRoles(program)
	starts := map[int]bool{}
	for _, element := range document.Elements {
		if element.HasRole(ExecutableBlockOpen) {
			starts[element.Span.Start] = true
		}
	}
	var lists, bodyless, literals int
	wanted := map[int]bool{}
	ast.Visit(program, func(node any) bool {
		var open lexer.Token
		switch node := node.(type) {
		case *ast.TryExpression:
			open = node.HandlersOpen
		case *ast.TryAssignmentStatement:
			open = node.HandlersOpen
		case *ast.StructLiteral:
			literals++
			return true
		case *ast.BlockStatement:
			if node.Token.Type == lexer.LBRACE {
				wanted[node.Token.ByteStart] = true
			}
			return true
		default:
			return true
		}
		if open.Type == lexer.LBRACE {
			lists++
			wanted[open.ByteStart] = true
			if !starts[open.ByteStart] {
				t.Fatal("parser-owned handler list lost block role", open)
			}
		} else {
			bodyless++
		}
		return true
	})
	if len(starts) != len(wanted) {
		t.Fatalf("executable roles include non-block braces: got %v; want %v", starts, wanted)
	}
	for start := range starts {
		if !wanted[start] {
			t.Fatal("configuration, value or field-pattern brace received executable-block role", start)
		}
	}
	if lists != 4 || bodyless != 1 || literals < 2 || document.Text() != source {
		t.Fatalf("lists=%d, bodyless=%d, literals=%d; source preserved=%v", lists, bodyless, literals, document.Text() == source)
	}
}
