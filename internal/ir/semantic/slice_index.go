package semantic

import "fmt"

// Slice indexing retains the original integer index and bounded source value.
// It forms a P15 Place; element read/write/borrow use the existing operations.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§50–58,81–82;
// rules/mlir/semantic-ir/sec_semantic_ir_slice_v1.md — §§15–20.
const (
	OpSliceIndexInBounds OpKind = "slice.index-in-bounds"
	OpSliceElementPlace  OpKind = "slice.element-place"
)

// verifySliceIndexOperation checks the predicate/Place contract without
// narrowing wide indexes or selecting physical offsets or element ownership.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§52,54–58,81–82.
func verifySliceIndexOperation(types *TypeTable, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 2 || len(op.Results) != 1 || len(op.Successors) != 0 {
		return fmt.Errorf("%s requires a slice, original index and one result", op.Kind)
	}
	source, ok := types.Lookup(values[op.Operands[0]].Type)
	signed, integer := semanticIntegerSigned(types, values[op.Operands[1]].Type)
	if !ok || source.Kind != TypeSharedSlice && source.Kind != TypeMutableSlice || !integer || signed != op.ArrayIndexSigned {
		return fmt.Errorf("%s requires a slice and original integer signedness", op.Kind)
	}
	if op.Kind == OpSliceIndexInBounds {
		if !typeHasKind(types, op.Results[0].Type, TypeBool) || op.Reference != nil {
			return fmt.Errorf("slice.index-in-bounds observes its source and produces bool")
		}
		return nil
	}
	place, ok := placeType(types, op.Results[0].Type)
	if !ok || place.Element != source.Element || place.Mutable != (source.Kind == TypeMutableSlice) {
		return fmt.Errorf("slice.element-place must preserve its source element and authority")
	}
	if err := verifyArrayBoundsMetadata(op); err != nil {
		return err
	}
	if err := verifyReferenceMetadata(op); err != nil {
		return err
	}
	if op.Integer != nil && op.Integer.Sign() < 0 {
		return fmt.Errorf("slice.element-place has a negative proven index")
	}
	return nil
}

// verifySliceIndexGuards confines runtime element Places to the matching
// predicate's true edge; failure is slice-index panic or canonical IndexError.
// Bounds proofs are resolved compiler facts, never reconstructed here.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§52–54,81–83;
// rules/mlir/semantic-ir/sec_semantic_ir_slice_v1.md — §§16–18.
func verifySliceIndexGuards(module *Module, fn *Function, blocks map[BlockID]*Block, values map[ValueID]Value, dom map[BlockID]map[BlockID]bool) error {
	type guard struct {
		op                      Operation
		block, success, failure BlockID
	}
	guards := map[ValueID]guard{}
	uses := map[ValueID]int{}
	graph := map[BlockID][]BlockID{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, operand := range operationOperands(op) {
				uses[operand]++
			}
			for _, successor := range op.Successors {
				graph[block.ID] = append(graph[block.ID], successor.Block)
			}
		}
	}
	for _, block := range fn.Blocks {
		for index, op := range block.Operations {
			if op.Kind != OpSliceIndexInBounds {
				continue
			}
			if index+1 != len(block.Operations)-1 {
				return fmt.Errorf("slice.index-in-bounds must immediately feed its bounds branch")
			}
			predicate := op.Results[0].ID
			branch := block.Operations[index+1]
			if uses[predicate] != 1 || branch.Kind != OpCondBranch || len(branch.Operands) != 1 || branch.Operands[0] != predicate || len(branch.Successors) != 2 {
				return fmt.Errorf("slice bounds predicate must feed one conditional branch")
			}
			if !validSequenceBoundsFailureBlock(module, blocks[branch.Successors[1].Block], values, "slice-index") {
				return fmt.Errorf("slice bounds predicate has invalid failure endpoint")
			}
			guards[predicate] = guard{op, block.ID, branch.Successors[0].Block, branch.Successors[1].Block}
		}
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind != OpSliceElementPlace || op.ArrayCheckKind != ArrayIndexRuntimeCheck {
				continue
			}
			g, ok := guards[op.ArrayGuard]
			if !ok || op.Operands[0] != g.op.Operands[0] || op.Operands[1] != g.op.Operands[1] {
				return fmt.Errorf("slice.element-place does not match its slice/index bounds guard")
			}
			if !dom[block.ID][g.block] || !dom[block.ID][g.success] || reachableAvoiding(graph, g.failure, block.ID, g.block) {
				return fmt.Errorf("slice.element-place is not confined to its bounds guard true edge")
			}
		}
	}
	return nil
}
