package sema

import "sec/internal/ast"

// ResolvedSliceLengthPlan identifies the resolved core operation, never a
// member or callee inferred by the backend from its spelling. Source syntax
// carries the operand; this detached plan carries its semantic type and the
// unsigned result contract.
// Rules: rules/compiler/semantic_ir.md — §126.14;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§37,47–49.
type ResolvedSliceLengthPlan struct {
	CompilerKnownID        string
	SourceType, ResultType Type
}

// recordSliceLengthPlan retains a validated Len/len resolution before its
// surrounding inference finishes. It creates no additional borrow or effect.
// Rules: rules/compiler/semantic_ir.md — §126.14;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§47–49.
func (a *Analyzer) recordSliceLengthPlan(expr, source ast.Expression) {
	plan := ResolvedSliceLengthPlan{SourceType: semanticSnapshotType(a.expressionTypes[source]), ResultType: semanticSnapshotType(a.types["uint"])}
	switch expr := expr.(type) {
	case *ast.MemberExpression:
		member, ok := a.CompilerKnownMemberAt(expr.Property.Token.File, expr.Property.Token.Line, expr.Property.Token.Column)
		if !ok || member.ID != "CKM-LEN-SLICE" {
			return
		}
		plan.CompilerKnownID = member.ID
	case *ast.CallExpression:
		// This producer is invoked only after compiler-known len validation.
		plan.CompilerKnownID = "CKF-LEN"
	default:
		return
	}
	if a.resolvedSliceLengthPlans == nil {
		a.resolvedSliceLengthPlans = map[ast.Expression]ResolvedSliceLengthPlan{}
	}
	a.resolvedSliceLengthPlans[expr] = plan
}

// ResolvedSliceLengthPlanOf reads existing facts without reevaluation, member
// lookup, borrow checking or identity allocation; nested types are detached.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§37,47–49.
func (a *Analyzer) ResolvedSliceLengthPlanOf(expr ast.Expression) (ResolvedSliceLengthPlan, bool) {
	if a == nil || expr == nil {
		return ResolvedSliceLengthPlan{}, false
	}
	plan, ok := a.resolvedSliceLengthPlans[expr]
	if !ok {
		return ResolvedSliceLengthPlan{}, false
	}
	plan.SourceType = semanticSnapshotType(plan.SourceType)
	plan.ResultType = semanticSnapshotType(plan.ResultType)
	return plan, true
}
