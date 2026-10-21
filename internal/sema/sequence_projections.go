package sema

import "sec/internal/ast"

// recordSequenceProjection retains the canonical source-relative projection
// before composition. Post-analysis consumers must not evaluate endpoints in
// a different callable's constant environment or allocate new identities.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§24–30,37;
// rules/compiler/compiler_analysis.md — §18(2,6).
func (a *Analyzer) recordSequenceProjection(expression ast.Expression, projection PlaceProjection) {
	if a.resolvedSequenceProjections == nil {
		a.resolvedSequenceProjections = map[ast.Expression]PlaceProjection{}
	}
	a.resolvedSequenceProjections[expression] = clonePlaceProjection(projection)
}
