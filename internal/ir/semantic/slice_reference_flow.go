package semantic

import "fmt"

// resolveForwardedSliceFacts preserves only unanimous reference facts across
// slice block arguments. Roots are solved to a fixed point, including anchored
// loop forwarding; unknown predecessors taint every dependent join. This does
// not infer new ranges, lifetime authority or reference identities at a phi.
// Rules: rules/compiler/semantic_ir.md — §§66,126.2–4,126.23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§44,63,83–86.
func resolveForwardedSliceFacts(types *TypeTable, fn *Function, blocks map[BlockID]*Block, sources map[ValueID]*ReferenceMetadata) {
	roots, unknown := cfgValueOrigins(fn, blocks)
	typesByValue := map[ValueID]TypeID{}
	for _, block := range fn.Blocks {
		for _, parameter := range block.Parameters {
			typesByValue[parameter.ID] = parameter.Type
		}
	}
	for id, candidates := range roots {
		typ, _ := types.Lookup(typesByValue[id])
		if unknown[id] || (typ.Kind != TypeSharedSlice && typ.Kind != TypeMutableSlice) {
			continue
		}
		var agreed *ReferenceMetadata
		for root := range candidates {
			if sources[root] == nil {
				agreed = nil
				break
			}
			if agreed == nil {
				agreed = sources[root]
			} else if !sameSliceValidityFacts(agreed, sources[root]) {
				agreed = nil
				break
			}
		}
		if agreed != nil {
			sources[id] = cloneReferenceMetadata(agreed)
		}
	}
}

// verifySliceElementReferenceFacts retains the granting slice's referent facts
// through P15 element/subfield borrowing. A new borrow may choose its own
// borrow/lifetime identity, but cannot change storage, epoch, pin or address
// dependencies. Ordinary borrows may derive implicitly through their Place.
// Rules: rules/compiler/semantic_ir.md — §126.17–19,23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§50–58,85–86,116.
func verifySliceElementReferenceFacts(op Operation, definitions map[ValueID]Operation) error {
	if op.Kind != OpReferenceBorrowShared && op.Kind != OpReferenceBorrowMutable {
		return nil
	}
	id := op.Operands[0]
	for {
		place := definitions[id]
		switch place.Kind {
		case OpPlaceField, OpPlaceArrayElement, OpPlaceUnionPayload:
			id = place.Operands[0]
			continue
		case OpSliceElementPlace:
			source, result := place.Reference, op.Reference
			if source == nil || result == nil || result.Borrow == source.Borrow {
				return fmt.Errorf("slice element borrow requires a distinct identity derived from its granting slice")
			}
			expected := cloneReferenceMetadata(source)
			expected.Borrow, expected.Lifetime, expected.SourceBorrow = result.Borrow, result.Lifetime, result.SourceBorrow
			if !sameSliceValidityFacts(expected, result) {
				return fmt.Errorf("slice element borrow changes its granting slice's reference facts")
			}
		}
		return nil
	}
}
