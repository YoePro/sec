package semantic

import (
	"fmt"
	"math/big"
	"slices"
)

// Slice construction consumes a Place or an existing bounded reference and a
// normalized start/length pair. It never loads or copies backing elements.
// Rules: rules/compiler/semantic_ir.md — §126.7–8;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§42–46.
const (
	OpSliceBorrowShared    OpKind = "slice.borrow-shared"
	OpSliceBorrowMutable   OpKind = "slice.borrow-mut"
	OpSliceReborrowShared  OpKind = "slice.reborrow-shared"
	OpSliceReborrowMutable OpKind = "slice.reborrow-mut"
)

// sliceConstructionKind identifies construction without conflating holder
// transport, element access or owning dynamic-array allocation.
// Rules: rules/compiler/semantic_ir.md — §126.6–10.
func sliceConstructionKind(kind OpKind) (construction, reborrow, mutable bool) {
	switch kind {
	case OpSliceBorrowShared:
		return true, false, false
	case OpSliceBorrowMutable:
		return true, false, true
	case OpSliceReborrowShared:
		return true, true, false
	case OpSliceReborrowMutable:
		return true, true, true
	}
	return false, false, false
}

// verifySliceBorrowOperation checks bounded construction and authority; the
// function-level check links the normalized tuple and complete source facts.
// Rules: rules/compiler/semantic_ir.md — §126.7–8;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§42–46,78–80.
func verifySliceBorrowOperation(types *TypeTable, op Operation, values map[ValueID]Value) error {
	_, reborrow, mutable := sliceConstructionKind(op.Kind)
	if len(op.Operands) != 3 || len(op.Results) != 1 || len(op.Successors) != 0 || op.SliceRange != nil {
		return fmt.Errorf("%s requires source, normalized start/length and one slice result", op.Kind)
	}
	for _, operand := range op.Operands[1:] {
		typ, ok := types.Lookup(values[operand].Type)
		if !ok || typ.Kind != TypeUint || typ.Name != "uint" || !typ.TargetSize {
			return fmt.Errorf("%s normalized start/length must be uint", op.Kind)
		}
	}
	source, _ := types.Lookup(values[op.Operands[0]].Type)
	element, writable := source.Element, source.Kind == TypeMutableSlice
	if reborrow {
		if source.Kind != TypeSharedSlice && source.Kind != TypeMutableSlice {
			return fmt.Errorf("%s requires a slice source", op.Kind)
		}
	} else {
		array, ok := types.Lookup(source.Element)
		if source.Kind != TypePlace || !ok || array.Kind != TypeArray {
			return fmt.Errorf("%s requires a fixed-array Place", op.Kind)
		}
		element, writable = array.Element, source.Mutable
	}
	kind, ownership := TypeSharedSlice, OwnershipSharedReference
	if mutable {
		kind, ownership = TypeMutableSlice, OwnershipMutableReference
	}
	result, ok := types.Lookup(op.Results[0].Type)
	if !ok || result.Kind != kind || result.Element != element || op.Results[0].Ownership != ownership || mutable && !writable {
		return fmt.Errorf("%s cannot change element type or increase source authority", op.Kind)
	}
	return verifyReferenceMetadata(op)
}

// verifySliceBorrowSource binds construction to the exact source extent and
// normalized SSA tuple. Existing range-check verification confines checked
// tuples to success. Reborrow facts retain the parent's temporal/storage
// contract with a distinct borrow, while holder dependency verification
// prevents the child (or its copies) from outliving its parent holder.
// Rules: rules/compiler/semantic_ir.md — §126.7–8,11;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§42–46,78–80,83–86.
func verifySliceBorrowSource(types *TypeTable, op Operation, values map[ValueID]Value,
	definitions map[ValueID]Operation, sources map[ValueID]*ReferenceMetadata) error {
	_, reborrow, _ := sliceConstructionKind(op.Kind)
	normalize, ok := definitions[op.Operands[1]]
	if !ok || normalize.Kind != OpSliceRangeNormalize && normalize.Kind != OpSliceRangeCheck ||
		len(normalize.Results) < 2 || normalize.Results[0].ID != op.Operands[1] || normalize.Results[1].ID != op.Operands[2] {
		return fmt.Errorf("%s requires the same normalized range start/length tuple", op.Kind)
	}
	extent := definitions[normalize.Operands[0]]
	if reborrow {
		if extent.Kind != OpSliceLength || len(extent.Operands) != 1 || extent.Operands[0] != op.Operands[0] {
			return fmt.Errorf("%s range must use the same source slice's length", op.Kind)
		}
		return verifySliceDerivedFacts(op, sources[op.Operands[0]])
	}
	sourceType, _ := types.Lookup(values[op.Operands[0]].Type)
	array, _ := types.Lookup(sourceType.Element)
	length, valid := new(big.Int).SetString(array.Length, 10)
	if !valid || extent.Kind != OpConstInt || extent.Integer == nil || extent.Integer.Cmp(length) != 0 {
		return fmt.Errorf("%s range extent must equal the fixed-array length", op.Kind)
	}
	for id := op.Operands[0]; ; {
		place := definitions[id]
		switch place.Kind {
		case OpPlaceField, OpPlaceArrayElement, OpPlaceUnionPayload:
			id = place.Operands[0]
			continue
		case OpReferenceDerefPlace:
			return verifySliceDerivedFacts(op, sources[place.Operands[0]])
		case OpPlaceValueRoot, OpPlaceStorageRoot, OpPlaceStaticRoot:
			if place.Place == nil || place.Place.Storage == 0 || op.Reference.AddressSpace != place.Place.AddressSpace ||
				!slices.Equal(op.Reference.OriginStorages, []uint32{place.Place.Storage}) {
				return fmt.Errorf("%s must preserve its source Place storage/address space", op.Kind)
			}
			if op.Reference.SourceBorrow != 0 || op.Reference.Validity != ReferenceProven {
				return fmt.Errorf("%s over owned storage requires a proven borrow without a source borrow", op.Kind)
			}
			return nil
		default:
			return fmt.Errorf("%s requires canonical source Place provenance", op.Kind)
		}
	}
}

// verifySliceDerivedFacts allows a new borrow/lifetime identity while retaining
// the complete source validity, storage, epoch, pin and address-space facts.
// Identity numbers are not lifetime durations; parent availability is checked
// along CFG paths rather than by ordering LifetimeIDs.
// Rules: rules/compiler/semantic_ir.md — §126.8;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§43–46,63,86.
func verifySliceDerivedFacts(op Operation, source *ReferenceMetadata) error {
	if source == nil || op.Reference.SourceBorrow != source.Borrow || op.Reference.Borrow == source.Borrow {
		return fmt.Errorf("%s requires a distinct borrow derived from its source", op.Kind)
	}
	expected := cloneReferenceMetadata(source)
	expected.Borrow, expected.Lifetime, expected.SourceBorrow = op.Reference.Borrow, op.Reference.Lifetime, source.Borrow
	if !sameSliceValidityFacts(expected, op.Reference) {
		return fmt.Errorf("%s changes its source validity/storage/epoch facts", op.Kind)
	}
	return nil
}
