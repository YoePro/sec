package semantic

import "fmt"

// Slice holder transfers preserve the complete bounded reference value,
// including length/range, storage and epoch. They do not observe or destroy
// backing elements and therefore need no dynamic-validity check themselves.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–41,45,63;
// rules/mlir/semantic-ir/sec_semantic_ir_slice_v1.md — §§9–10,21.
const (
	OpSliceCopyShared  OpKind = "slice.copy-shared"
	OpSliceMoveMutable OpKind = "slice.move-mut"
)

// verifySliceTransferOperation checks explicit shared copy versus exclusive
// move without selecting an element-copy/destruction or descriptor layout.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–41.
func verifySliceTransferOperation(types *TypeTable, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 1 || len(op.Results) != 1 || len(op.Successors) != 0 || op.ReferenceGuard != 0 || op.SliceRange != nil {
		return fmt.Errorf("%s requires one holder operand/result without range changes or validity observation", op.Kind)
	}
	source, ok := types.Lookup(values[op.Operands[0]].Type)
	kind, ownership := TypeSharedSlice, OwnershipSharedReference
	if op.Kind == OpSliceMoveMutable {
		kind, ownership = TypeMutableSlice, OwnershipMutableReference
	}
	if !ok || source.Kind != kind || op.Results[0].Type != values[op.Operands[0]].Type || op.Results[0].Ownership != ownership {
		return fmt.Errorf("%s requires matching slice authority/type and holder ownership", op.Kind)
	}
	if err := verifyReferenceMetadata(op); err != nil {
		return err
	}
	if op.Reference.SourceBorrow != op.Reference.Borrow {
		return fmt.Errorf("%s must preserve its source borrow", op.Kind)
	}
	return nil
}

// verifySliceHolderFacts prevents copy/move/end markers and element Places
// from changing the source's borrow, lifetime, storage or temporal contract.
// Actual bounded-value transport is intrinsic to the copy/move operation;
// it cannot introduce another length or range operand.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§40–41,63,82.
func verifySliceHolderFacts(op Operation, source *ReferenceMetadata) error {
	if source == nil {
		return fmt.Errorf("%s source has no reference facts", op.Kind)
	}
	expected := cloneReferenceMetadata(source)
	if op.Kind == OpSliceCopyShared || op.Kind == OpSliceMoveMutable {
		expected.SourceBorrow = expected.Borrow
	}
	if !sameSliceValidityFacts(expected, op.Reference) {
		return fmt.Errorf("%s changes its source holder's reference facts", op.Kind)
	}
	return nil
}
