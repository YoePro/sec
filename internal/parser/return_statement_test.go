package parser

import (
	"os"
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// TestReturnMoveMarkerBelongsToStatement verifies the grammar's optional
// terminal marker, including expressions that are not named move sources.
// Rules: rules/foundations/grammar.md — "Return statement";
// rules/declarations/functions.md — §14.
func TestReturnMoveMarkerBelongsToStatement(t *testing.T) {
	for _, fixture := range []string{"../../testdata/ownership/explicit_return_valid.sec", "../../testdata/build/native/explicit_return.sec"} {
		source, err := os.ReadFile(fixture)
		if err != nil {
			t.Fatal(err)
		}
		p := New(lexer.NewWithFile(string(source), fixture))
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatal(p.Errors())
		}
		found := 0
		for _, statement := range program.Statements {
			function, ok := statement.(*ast.FunctionDeclaration)
			if !ok {
				continue
			}
			for _, statement := range function.Body.Statements {
				returned, ok := statement.(*ast.ReturnStatement)
				if !ok {
					continue
				}
				found++
				if returned.MoveToken.Type != lexer.MOVE_ASSIGN || returned.Value == nil {
					t.Fatalf("marker or expression missing: %#v", returned)
				}
				if prefix, ok := returned.Value.(*ast.PrefixExpression); ok && prefix.Operator == "<-" {
					t.Fatal("terminal marker incorrectly parsed as unary move")
				}
				if function.Name.Value == "Scalar" || function.Name.Value == "Sum" {
					if _, ok := returned.Value.(*ast.InfixExpression); !ok {
						t.Fatalf("marker must precede the whole expression: %T", returned.Value)
					}
				}
			}
		}
		if found == 0 {
			t.Fatal("fixture has no marked returns")
		}
	}
}

// A dangling marker is never a bare return and recovery preserves the next
// declaration (rules/compiler/parser_recovery.md — "Expression recovery").
func TestReturnMoveMarkerRequiresValue(t *testing.T) {
	source, err := os.ReadFile("../../testdata/parser/explicit_return_missing_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := New(lexer.New(string(source)))
	program := p.ParseProgram()
	if len(p.Errors()) == 0 {
		t.Fatal("accepted a marker without an expression")
	}
	if functionNamed(program, "Next") == nil {
		t.Fatal("lost following declaration")
	}
}
