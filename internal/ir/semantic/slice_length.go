package semantic

import (
	"fmt"
	"slices"
)

// OpSliceLength observes the slice's element count without reading elements
// or transferring the slice holder. Both .Len and len(...) denote this uint
// observation; there is no signed-length operation or hidden conversion.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§47–49,110–111;
// rules/corrections/applied/md051-md074-md088-length-numeric-string-correction-20261017.md — §1.
const OpSliceLength OpKind = "slice.length"

// directSafeReferenceType identifies safe reference values for validity
// checks without treating a slice as a direct reference to one element.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§31–32,44,84.
func directSafeReferenceType(types *TypeTable, id TypeID) bool {
	typ, ok := types.Lookup(id)
	return ok && (typ.Kind == TypeReference || typ.Kind == TypeSharedSlice || typ.Kind == TypeMutableSlice)
}

// verifySliceLengthOperation requires one bounded slice and its exact uint
// count. Raw pointers, direct element references, arrays and signed lengths
// are different semantic operations.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§47–49,110–111.
func verifySliceLengthOperation(types *TypeTable, op Operation, values map[ValueID]Value) error {
	if len(op.Operands) != 1 || len(op.Results) != 1 || len(op.Successors) != 0 {
		return fmt.Errorf("slice.length requires one slice operand and one uint result")
	}
	source, ok := types.Lookup(values[op.Operands[0]].Type)
	result, resultOK := types.Lookup(op.Results[0].Type)
	if !ok || source.Kind != TypeSharedSlice && source.Kind != TypeMutableSlice ||
		!resultOK || result.Kind != TypeUint || result.Name != "uint" || !result.TargetSize {
		return fmt.Errorf("slice.length requires a shared/mutable slice and target-sized uint result")
	}
	if op.Reference != nil {
		return fmt.Errorf("slice.length observes its operand; it does not create a new reference")
	}
	return nil
}

// verifySliceObservationValidity consumes the canonical P15 source/guard facts.
// Length and slice indexing share this temporal gate. A dynamic guard must validate the same borrow and epoch, and its successful
// branch must dominate the observation with no failure-path bypass. The stale
// endpoint stays InvalidReferenceGeneration independently of spatial try flow.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§44–49,50–58,81–84,110–111;
// rules/memory/reference_model.md — §30.
func verifySliceObservationValidity(op Operation, block BlockID, blocks map[BlockID]*Block,
	dom map[BlockID]map[BlockID]bool, definitions map[ValueID]Operation,
	definitionBlocks map[ValueID]BlockID, sources map[ValueID]*ReferenceMetadata) error {
	source := sources[op.Operands[0]]
	if source == nil {
		return fmt.Errorf("%s source has no canonical reference facts", op.Kind)
	}
	if err := verifyReferenceMetadata(Operation{Kind: op.Kind, Reference: source}); err != nil {
		return err
	}
	if source.Validity != ReferenceDynamicEpoch {
		return nil
	}
	guard, ok := definitions[op.ReferenceGuard]
	if !ok || guard.Kind != OpReferenceIsValid || len(guard.Operands) != 1 || guard.Operands[0] != op.Operands[0] ||
		!sameSliceValidityFacts(source, guard.Reference) {
		return fmt.Errorf("%s requires a validity guard for the same source borrow and epoch", op.Kind)
	}
	guardBlock := blocks[definitionBlocks[op.ReferenceGuard]]
	if guardBlock == nil || len(guardBlock.Operations) == 0 {
		return fmt.Errorf("%s validity guard has no block", op.Kind)
	}
	branch := guardBlock.Operations[len(guardBlock.Operations)-1]
	if branch.Kind != OpCondBranch || len(branch.Operands) != 1 || branch.Operands[0] != op.ReferenceGuard || len(branch.Successors) != 2 {
		return fmt.Errorf("%s validity guard must select success and stale-reference failure", op.Kind)
	}
	graph := map[BlockID][]BlockID{}
	for id, b := range blocks {
		for _, operation := range b.Operations {
			for _, successor := range operation.Successors {
				graph[id] = append(graph[id], successor.Block)
			}
		}
	}
	success, failure := branch.Successors[0].Block, branch.Successors[1].Block
	if !dom[block][guardBlock.ID] || !dom[block][success] || reachableAvoiding(graph, failure, block, guardBlock.ID) {
		return fmt.Errorf("%s is not dominated by its source validity guard's success edge", op.Kind)
	}
	return nil
}

// sameSliceValidityFacts ties a check to the complete existing P15 identity;
// matching only the slice SSA operand cannot justify another epoch or borrow.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§44–46,84.
func sameSliceValidityFacts(source, guard *ReferenceMetadata) bool {
	return source != nil && guard != nil && source.Borrow == guard.Borrow && source.Lifetime == guard.Lifetime &&
		source.SourceBorrow == guard.SourceBorrow && source.Validity == guard.Validity &&
		source.InvalidationDomain == guard.InvalidationDomain && source.EpochDependency == guard.EpochDependency &&
		source.AddressSpace == guard.AddressSpace && source.AddressStability == guard.AddressStability &&
		source.Pinned == guard.Pinned && source.Provenance == guard.Provenance &&
		source.ProofProvenance == guard.ProofProvenance && slices.Equal(source.OriginStorages, guard.OriginStorages)
}
