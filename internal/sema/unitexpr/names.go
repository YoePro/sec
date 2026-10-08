// Package unitexpr provides source-level unit expression traversal without
// depending on Analyzer or resolved semantic types.
package unitexpr

import "sec/internal/ast"

// FirstName returns the first matching named factor in source order, including
// factors inside groups and powers. Identity factors introduce no names.
// Rules: rules/types/units.md — Structural unit expressions; future unit polymorphism.
func FirstName(expression *ast.UnitExpression, matches func(string) bool) *ast.UnitExpression {
	if expression == nil {
		return nil
	}
	if expression.Kind == ast.UnitExpressionName && matches(expression.Name) {
		return expression
	}
	if found := FirstName(expression.Left, matches); found != nil {
		return found
	}
	return FirstName(expression.Right, matches)
}
