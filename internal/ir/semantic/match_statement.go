package semantic

import (
	"fmt"
	"sec/internal/ast"
	"sec/internal/sema"
)

// buildMatchStatementArm emits the selected statement arm, consumes its
// expression result through the ordinary discard producer, and runs arm cleanup
// before branching to the common continuation. Terminating arms have no branch.
// Rules: rules/control-flow/flowcontrol_match.md §§18-19,23;
// rules/control-flow/discard.md §§9-10; rules/memory/destruction.md §§6,8;
// rules/mlir/packages/sec-mlir-dialect_package17.md §§63,111.
func (fb *functionBuilder) buildMatchStatementArm(arm *ast.MatchArm, plan sema.ResolvedMatchArm, continuation *Block, matchID MatchID) error {
	if arm.ReturnBody != nil {
		return fb.buildReturn(arm.ReturnBody)
	}
	if arm.BlockBody != nil {
		// A match payload's home list is its arm body (MD-065).
		fb.pendingHomes = append(fb.pendingHomes, fb.armPayloadBindings...)
		if err := fb.buildScope(arm.BlockBody); err != nil {
			return err
		}
	} else if arm.Body != nil {
		armType, ok := fb.owner.analyzer.ResolvedTypeOf(arm.Body)
		if !ok {
			return fb.unsupported("untyped match statement arm", arm.Token)
		}
		// A statement arm has ordinary implicit-discard semantics. Reuse the
		// terminal ownership producer instead of leaving a non-trivial SSA
		// result unused; it must end before full-expression and arm cleanup.
		implicitResult := false
		switch arm.Body.(type) {
		case *ast.CallExpression, *ast.NewExpression, *ast.TryExpression:
			implicitResult = true
		}
		if implicitResult && armType.Kind != sema.VoidType && armType.Kind != sema.NeverType {
			if err := fb.discardExpression(arm.Body, DiscardImplicitCallResult, expressionToken(arm.Body)); err != nil {
				return err
			}
		} else {
			// Match grants no new implicit consumption of an arbitrary
			// stored value. Preserve ordinary expression support boundaries.
			if armType.Kind != sema.VoidType && armType.Kind != sema.NeverType &&
				(sema.CopyClassificationOf(armType) != sema.CopyTrivial || !sema.TriviallyDestructible(armType)) {
				return fb.unsupported("non-call statement expression result requiring ownership", expressionToken(arm.Body))
			}
			armTypeID, err := fb.owner.internType(armType)
			if err != nil {
				return err
			}
			if _, err := fb.buildExpr(arm.Body, armTypeID); err != nil {
				return err
			}
		}
		if err := fb.endFullExpression(); err != nil {
			return err
		}
	}
	if fb.current == nil {
		return nil
	}
	if continuation == nil {
		return fmt.Errorf("continuing match arm %d has no continuation", plan.SourceIndex)
	}
	fb.runScopeCleanup(sema.CleanupExitFallthrough, arm.Token)
	if err := fb.endArmTemporaries(fb.armTemporaryDepth); err != nil {
		return err
	}
	fb.emit(Operation{
		Kind: OpBranch, Successors: []BranchTarget{{Block: continuation.ID}}, MatchID: matchID,
		MatchArmIndex: plan.SourceIndex, MatchStage: "body-exit",
		MatchPatternKind: string(plan.PatternKind), Location: location(arm.Token),
	})
	fb.current = nil
	return nil
}
