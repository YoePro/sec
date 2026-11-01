package sema

import "sec/internal/ast"

// membershipDropsReferenceOrigins uses the resolved operator, not its spelling,
// to distinguish observing borrowed collection elements from returning a carrier
// that retains those borrows. Its bool result cannot contain a local or
// arm-scoped reference; operand validity and borrow checks still run normally.
// Rules: rules/foundations/operators.md — "Membership expression", §A.14;
// rules/memory/lifetime_analysis.md — §§1–4;
// rules/memory/ownership.md — §28.1.
func (a *Analyzer) membershipDropsReferenceOrigins(expr *ast.InfixExpression) bool {
	operator, ok := a.ResolvedOperatorOf(expr)
	return ok && operator.ResultType.Kind == BoolType && (operator.Kind == ResolvedMembershipCompare || operator.Kind == ResolvedNegatedMembershipCompare)
}
