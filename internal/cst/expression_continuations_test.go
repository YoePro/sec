package cst

import (
	"os"
	"reflect"
	"testing"

	"sec/internal/lexer"
	"sec/internal/parser"
)

// Return/if continuations retain parser-owned anchors without duplicating facts
// through comment attachment owners or counting an inline-opened call twice.
// Applying roles again is lossless and idempotent.
// Rules: rules/tooling/formatter.md — §5, §6(3), §11(3), §29.
func TestExpressionContinuationRelations(t *testing.T) {
	source, err := os.ReadFile("../../testdata/formatter/expression_continuations.sec")
	if err != nil {
		t.Fatal(err)
	}
	p := parser.New(lexer.New(string(source)))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	d := Build(string(source), "")
	d.ApplyProgramRoles(program)
	seen := map[ExpressionContinuation]bool{}
	whitespaceLines := 0
	conditionLines := 0
	for _, continuation := range d.ExpressionContinuations {
		if seen[continuation] {
			t.Fatal("duplicate return continuation", continuation)
		}
		seen[continuation] = true
		keyword, element := d.Elements[continuation.Keyword], d.Elements[continuation.Element]
		if (keyword.Token.Type != lexer.RETURN && keyword.Token.Type != lexer.IF) || element.Token.Line <= keyword.Token.Line {
			t.Fatal("invalid continuation anchor")
		}
		if keyword.Token.Line == 2 {
			whitespaceLines++
		}
		if keyword.Token.Type == lexer.IF {
			conditionLines++
		}
		if keyword.Token.Line == 16 && element.Token.Line == 18 {
			t.Fatal("inline-opened call closer counted as continuation")
		}
	}
	if whitespaceLines != 9 {
		t.Fatal("missing whitespace return operands", whitespaceLines)
	}
	if conditionLines != 7 {
		t.Fatal("missing or excessive condition continuations", conditionLines)
	}
	before := append([]ExpressionContinuation(nil), d.ExpressionContinuations...)
	d.ApplyProgramRoles(program)
	if !reflect.DeepEqual(before, d.ExpressionContinuations) || d.Text() != string(source) {
		t.Fatal("return relation publication is not lossless/idempotent")
	}
}
