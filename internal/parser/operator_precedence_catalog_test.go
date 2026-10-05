package parser

import (
	"testing"

	"sec/internal/ast"
	"sec/internal/lexer"
)

// Every adjacent binary precedence category groups through the catalog order
// consumed by the Pratt parser. Semantic validity of the placeholder operands
// is intentionally outside this syntax test.
//
// Rules:
//   - rules/foundations/operators.md — "Canonical precedence"
//   - rules/foundations/operators.md — Appendix A.2 "Create one precedence definition"
func TestCanonicalAdjacentBinaryPrecedence(t *testing.T) {
	tests := []struct {
		weak   string
		strong string
	}{
		{"||", "&&"},
		{"&&", "|"},
		{"|", "^"},
		{"^", "&"},
		{"&", "=="},
		{"==", "<"},
		{"<", "<<"},
		{"<<", "+"},
		{"+", "*"},
	}
	for _, test := range tests {
		t.Run(test.weak+"_before_"+test.strong, func(t *testing.T) {
			expression := parsePrecedenceTestExpression(t, "a "+test.weak+" b "+test.strong+" c")
			root, ok := expression.(*ast.InfixExpression)
			if !ok || root.Operator != test.weak {
				t.Fatalf("root = %#v, want weak operator %q", expression, test.weak)
			}
			right, ok := root.Right.(*ast.InfixExpression)
			if !ok || right.Operator != test.strong {
				t.Fatalf("right = %#v, want strong operator %q", root.Right, test.strong)
			}
		})
	}
}

// Ordinary binary operators, including contextual matrix x, remain
// left-associative at one precedence level.
//
// Rules:
//   - rules/foundations/operators.md — "Left-associative binary operators"
//   - rules/foundations/operators.md — "Matrix multiplication with contextual `x`"
func TestCanonicalBinaryAssociativity(t *testing.T) {
	for _, operator := range []string{"-", "<<", "x"} {
		t.Run(operator, func(t *testing.T) {
			expression := parsePrecedenceTestExpression(t, "a "+operator+" b "+operator+" c")
			root, ok := expression.(*ast.InfixExpression)
			if !ok || root.Operator != operator {
				t.Fatalf("root = %#v, want %q", expression, operator)
			}
			left, ok := root.Left.(*ast.InfixExpression)
			if !ok || left.Operator != operator {
				t.Fatalf("left = %#v, want left-associated %q", root.Left, operator)
			}
		})
	}
}

func parsePrecedenceTestExpression(t *testing.T, expression string) ast.Expression {
	t.Helper()
	p := New(lexer.New("fn F() void { let value := " + expression + " }"))
	program := p.ParseProgram()
	checkParserErrors(t, p)
	fn := program.Statements[0].(*ast.FunctionDeclaration)
	return fn.Body.Statements[0].(*ast.LetStatement).Value
}
