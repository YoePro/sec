package semantic

import (
	"sec/internal/ast"
	"sec/internal/sema"
)

// trySuccessType permits an explicitly constructed slice as a try success
// value without enabling slice parameters, returns or holder transfers.
// Rules: rules/compiler/semantic_ir.md — §126.6–8;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§21–23,31–32,76.
func (fb *functionBuilder) trySuccessType(source sema.Type) (TypeID, error) {
	if fb.owner.maxPackage < sliceLoweringPackage || !sourceSliceType(source) {
		return fb.owner.internType(source)
	}
	mutable := source.ReferenceMutable
	if source.Kind == sema.ReferenceType {
		source = *source.Element
	}
	element, err := fb.owner.internType(*source.Element)
	if err != nil {
		return 0, err
	}
	return fb.owner.module.Types.InternSlice(element, mutable)
}

// completeSliceTryMerge preserves the original successful slice holder when
// every failure path terminates or propagates. The ordinary try dispatcher
// retains its metadata and continuation; a single incoming success edge needs
// no phi, copy, move or new borrow. Recovery joins remain the separately owned
// slice holder/CFG work and must not discard their reference or origin facts.
// Rules: rules/errors/errorhandling.md — §§15–16,21.1,35;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§21–23,39–41,60–67,83.
func (fb *functionBuilder) completeSliceTryMerge(expr *ast.TryExpression, merge *Block, success builtValue) (builtValue, error) {
	var incoming *BranchTarget
	for _, block := range fb.fn.Blocks {
		for i := range block.Operations {
			op := &block.Operations[i]
			for j := range op.Successors {
				edge := &op.Successors[j]
				if edge.Block != merge.ID {
					continue
				}
				if incoming != nil || len(edge.Arguments) != 1 || edge.Arguments[0] != success.id {
					// A recovery value joins slice holders: the merge keeps its
					// block parameter, and each use reads the joined holder with
					// Sema's facts, which cover every incoming origin (P17;
					// lifetime_analysis.md § 11).
					if fb.owner.maxPackage >= ownershipLoweringPackage && len(merge.Parameters) == 1 {
						return builtValue{id: merge.Parameters[0].ID, typ: merge.Parameters[0].Type}, nil
					}
					return builtValue{}, fb.unsupported("slice recovery holder joins (P16-26/P16-36)", expr.Token)
				}
				incoming = edge
			}
		}
	}
	if incoming == nil || fb.references[success.id] == nil {
		// An unwrapped Arena Ok holder states its facts only on the unwrap:
		// from P17 the merge keeps its block parameter and each use reads it
		// with Sema's holder facts, checked against the incoming facts like
		// a recovery join (package15 § 101; P19 § 35).
		if fb.owner.maxPackage >= ownershipLoweringPackage && len(merge.Parameters) == 1 && incoming != nil && fb.okUnwrapped(incoming.Arguments[0]) {
			return builtValue{id: merge.Parameters[0].ID, typ: merge.Parameters[0].Type}, nil
		}
		return builtValue{}, fb.unsupported("slice try success without its original holder facts", expr.Token)
	}
	incoming.Arguments = nil
	merge.Parameters = nil
	return success, nil
}
