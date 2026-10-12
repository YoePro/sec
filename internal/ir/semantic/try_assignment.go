package semantic

import (
	"fmt"

	"sec/internal/ast"
	"sec/internal/sema"
)

// buildTryAssignment consumes Sema's ResolvedTryAssignment for a fallible
// assignment. Its fallible step is either a `try set` property setter or the
// runtime contract check of a constrained named type; Semantic IR represents
// neither yet, so the statement is rejected with the resolved plan (edge kind,
// error channel and fallible step) instead of being lowered as a plain store
// or rebuilt from handler syntax. Once the step produces a Result, the success
// edge stores and the failure edge reuses the try propagation and local
// handler lowering.
//
// Rules:
//   - rules/errors/errorhandling.md — §23 "Fallible assignment", §35 "Semantic IR and lowering requirements"
//   - rules/compiler/semantic_ir.md — "Unsupported lowerings"
func (fb *functionBuilder) buildTryAssignment(stmt *ast.TryAssignmentStatement) error {
	fact, ok := fb.owner.analyzer.ResolvedTryAssignmentOf(stmt)
	if !ok || stmt.Assignment == nil {
		return fb.unsupported("unresolved try assignment", stmt.Token)
	}
	if fact.TestBoundary || fact.HandlerPlan.ResidualTestBoundary {
		// errorhandling.md §41: the test invocation has no Semantic IR boundary.
		return fb.unsupported("try assignment propagation to a test boundary", stmt.Token)
	}
	edge := "propagating to " + sema.TypeDisplayName(fact.EnclosingResultType)
	if fact.Kind == sema.ResolvedTryAssignmentHandled {
		edge = "with local handlers"
	}
	step := "constrained-type contract check"
	if access, isProperty := fb.owner.analyzer.ResolvedPropertyAccessOf(stmt.Assignment.Target); isProperty && access.Kind == sema.PropertyFallibleWrite {
		step = "fallible property setter " + access.Name
	}
	return fb.unsupported(fmt.Sprintf("try assignment through %s (%s error %s)", step, edge, sema.TypeDisplayName(fact.ErrorType)), stmt.Token)
}
