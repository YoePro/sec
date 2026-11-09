package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// buildMatchExpressionArm builds a selected expression-arm result, including
// Sema-resolved moves and registration cancellation, before cleanup and merge.
// Rules: rules/control-flow/flowcontrol_match.md §§20-21.2;
// rules/memory/destruction.md §§6.2,8.
func (fb *functionBuilder) buildMatchExpressionArm(arm *ast.MatchArm, plan sema.ResolvedMatchArm, merge *Block, resultType TypeID, matchID MatchID) error {
	if arm.ReturnBody != nil {
		return fb.buildReturn(arm.ReturnBody)
	}
	if arm.BlockBody != nil {
		return fb.buildMatchExpressionBlockArm(arm, plan, merge, resultType, matchID)
	}
	if arm.Body == nil || plan.Flow != sema.MatchArmProducesValue {
		return fb.unsupported("non-expression match arm", arm.Token)
	}
	depth := len(fb.temporaries)
	value, err := fb.buildExpr(arm.Body, resultType)
	if err != nil {
		return err
	}
	return fb.finishMatchExpressionArm(arm, plan, value, depth, merge, matchID)
}

// buildMatchExpressionBlockArm lowers a block arm of a match expression. A
// value-producing block yields its final expression; the value is built first
// and leaving the block then runs its owned-value cleanup, as a value-producing
// handler block does (flowcontrol_match.md; destruction.md § 6.2; P17-42). A
// block that leaves by return, loop control or termination never reaches the
// merge.
// Rules: rules/control-flow/flowcontrol_match.md §§21-21.2;
// rules/memory/destruction.md §6.2;
// rules/mlir/packages/sec-mlir-dialect_package17.md §§63,111.
func (fb *functionBuilder) buildMatchExpressionBlockArm(arm *ast.MatchArm, plan sema.ResolvedMatchArm, merge *Block, resultType TypeID, matchID MatchID) error {
	statements := arm.BlockBody.Statements
	popScope := fb.enterBuildScope(arm.BlockBody)
	defer popScope()
	fb.pendingHomes = append(fb.pendingHomes, fb.armPayloadBindings...)
	// Sema leaves a block arm that falls through without a flow; in a match
	// expression its final expression is the arm's value.
	if plan.Flow != sema.MatchArmProducesValue && plan.Flow != "" {
		if err := fb.buildStatements(statements); err != nil {
			return err
		}
		if fb.current != nil {
			return fb.unsupported("match expression block arm without a value", arm.Token)
		}
		return nil
	}
	if len(statements) == 0 {
		return fb.unsupported("match expression block arm without a final expression", arm.Token)
	}
	final, ok := statements[len(statements)-1].(*ast.ExpressionStatement)
	if !ok || final.Expression == nil {
		return fb.unsupported("match expression block arm without a final expression", arm.Token)
	}
	if err := fb.buildStatements(statements[:len(statements)-1]); err != nil {
		return err
	}
	if fb.current == nil {
		return nil
	}
	depth := len(fb.temporaries)
	value, err := fb.buildExpr(final.Expression, resultType)
	if err != nil {
		return err
	}
	return fb.finishMatchExpressionArm(arm, plan, value, depth, merge, matchID)
}

// finishMatchExpressionArm hands the already-built result to the merge only
// after its source transfer has committed. Arm-body locals and the separate
// owning payload binding have distinct cleanup scopes; both must end, in that
// order, without destroying the transferred result.
// Rules: rules/control-flow/flowcontrol_match.md §§12,18,21.2(1)-(3);
// rules/memory/destruction.md §§6.2,8;
// rules/mlir/packages/sec-mlir-dialect_package17.md §§38,40,55,63,111.
func (fb *functionBuilder) finishMatchExpressionArm(arm *ast.MatchArm, plan sema.ResolvedMatchArm, value builtValue, depth int, merge *Block, matchID MatchID) error {
	if err := fb.endRegionTemporaries(depth); err != nil {
		return err
	}
	if arm.BlockBody != nil {
		fb.runScopeCleanup(sema.CleanupExitFallthrough, arm.BlockBody.Token)
	}
	fb.runScopeCleanup(sema.CleanupExitFallthrough, arm.Token)
	if err := fb.endArmTemporaries(fb.armTemporaryDepth); err != nil {
		return err
	}
	fb.emit(Operation{Kind: OpBranch, Successors: []BranchTarget{{Block: merge.ID, Arguments: []ValueID{value.id}}}, MatchID: matchID, MatchArmIndex: plan.SourceIndex, MatchStage: "body-exit", MatchPatternKind: string(plan.PatternKind), Location: location(arm.Token)})
	fb.current = nil
	return nil
}
