package unitexpr

import (
	"testing"

	"sec/internal/ast"
)

// TestFirstNameTraversesSourceFactors verifies source order through grouped powers.
// Rules: rules/types/units.md — Structural unit expressions.
func TestFirstNameTraversesSourceFactors(t *testing.T) {
	first := &ast.UnitExpression{Kind: ast.UnitExpressionName, Name: "U"}
	last := &ast.UnitExpression{Kind: ast.UnitExpressionName, Name: "U"}
	tree := &ast.UnitExpression{Kind: ast.UnitExpressionDivide,
		Left:  &ast.UnitExpression{Kind: ast.UnitExpressionGroup, Left: &ast.UnitExpression{Kind: ast.UnitExpressionPower, Left: first, Exponent: 2}},
		Right: last}
	if got := FirstName(tree, func(name string) bool { return name == "U" }); got != first {
		t.Fatal(got)
	}
	if got := FirstName(tree, func(name string) bool { return name == "m" }); got != nil {
		t.Fatal(got)
	}
	if got := FirstName(nil, func(string) bool { t.Fatal("nil visits predicate"); return true }); got != nil {
		t.Fatal(got)
	}
}
