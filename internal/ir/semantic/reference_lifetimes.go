package semantic

import "fmt"

// sliceHolderAliasIDs follows CFG parameter forwarding of the same slice
// holder. Copy/move results start another holder; loop parameters may be
// redefined by a later iteration. Direct P15 references retain their existing
// exact-SSA boundary rules. Derived slice element/sub-Places also become
// unavailable when their source holder ends or moves.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–41,50–58,63;
// rules/compiler/semantic_ir.md — §66.
func sliceHolderAliasIDs(ended ValueID, fn *Function, blocks map[BlockID]*Block, values map[ValueID]Value, types *TypeTable) map[ValueID]bool {
	aliases := map[ValueID]bool{ended: true}
	typ, ok := types.Lookup(values[ended].Type)
	if !ok || typ.Kind != TypeSharedSlice && typ.Kind != TypeMutableSlice {
		return aliases
	}
	graph := map[ValueID][]ValueID{}
	holders := map[ValueID]bool{}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			// A Place is a derived access path, not another slice holder.
			// Invalidation flows from the holder to the Place, never back.
			switch op.Kind {
			case OpSliceElementPlace, OpPlaceField, OpPlaceArrayElement, OpPlaceUnionPayload:
				if len(op.Operands) != 0 && len(op.Results) == 1 {
					graph[op.Operands[0]] = append(graph[op.Operands[0]], op.Results[0].ID)
				}
			}
			if op.Kind == OpSliceCopyShared || op.Kind == OpSliceMoveMutable {
				for _, result := range op.Results {
					holders[result.ID] = true
				}
			}
			for _, successor := range op.Successors {
				target := blocks[successor.Block]
				for index, argument := range successor.Arguments {
					parameter := target.Parameters[index]
					if parameter.Type == values[ended].Type && values[argument].Type == parameter.Type {
						graph[argument] = append(graph[argument], parameter.ID)
						graph[parameter.ID] = append(graph[parameter.ID], argument)
					}
				}
			}
		}
	}
	queue := []ValueID{ended}
	for len(queue) != 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range graph[current] {
			if aliases[next] || holders[next] && next != ended {
				continue
			}
			aliases[next] = true
			queue = append(queue, next)
		}
	}
	return aliases
}

// verifyNoUseAfterEndBorrow rejects any use of a reference reachable after
// its ref.end-borrow or mutable slice move, in the same block or along any CFG path. A path that
// reaches the reference's definition again (a loop re-entering the borrow)
// starts a new borrow instance and is not followed further. The marker must
// name the BorrowID and LifetimeID of the reference it ends.
// Rules: rules/mlir/packages/sec-mlir-dialect_package15.md — §63;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–41,63.
func verifyNoUseAfterEndBorrow(fn *Function, blocks map[BlockID]*Block, values map[ValueID]Value, types *TypeTable) error {
	definitions := map[ValueID]*ReferenceMetadata{}
	for _, parameter := range fn.Parameters {
		definitions[parameter.Value.ID] = parameter.Reference
	}
	for _, b := range fn.Blocks {
		for _, op := range b.Operations {
			for _, result := range op.Results {
				definitions[result.ID] = op.Reference
			}
		}
	}
	for _, b := range fn.Blocks {
		for index, op := range b.Operations {
			if op.Kind != OpReferenceEndBorrow && op.Kind != OpSliceMoveMutable {
				continue
			}
			ended := op.Operands[0]
			if defined := definitions[ended]; defined != nil && op.Reference != nil && (defined.Borrow != op.Reference.Borrow || defined.Lifetime != op.Reference.Lifetime) {
				return fmt.Errorf("ref.end-borrow of %%%d names borrow #%d lifetime #%d, not the reference's borrow #%d lifetime #%d", ended, op.Reference.Borrow, op.Reference.Lifetime, defined.Borrow, defined.Lifetime)
			}
			aliases := sliceHolderAliasIDs(ended, fn, blocks, values, types)
			boundary := "resolved borrow end"
			if op.Kind == OpSliceMoveMutable {
				boundary = "mutable slice move"
			}
			redefines := func(operation Operation) bool {
				for _, result := range operation.Results {
					if result.ID == ended {
						return true
					}
				}
				return false
			}
			uses := func(operation Operation) bool {
				for _, operand := range operationOperands(operation) {
					if aliases[operand] {
						return true
					}
				}
				return false
			}
			for _, later := range b.Operations[index+1:] {
				if uses(later) {
					return fmt.Errorf("reference %%%d is used after its %s", ended, boundary)
				}
			}
			visited := map[BlockID]bool{}
			queue := []BlockID{}
			for _, successor := range b.Operations[len(b.Operations)-1].Successors {
				queue = append(queue, successor.Block)
			}
			for len(queue) > 0 {
				id := queue[0]
				queue = queue[1:]
				if visited[id] || blocks[id] == nil {
					continue
				}
				visited[id] = true
				fresh := false
				for _, parameter := range blocks[id].Parameters {
					if parameter.ID == ended {
						fresh = true
					}
				}
				if fresh {
					continue
				}
				for _, operation := range blocks[id].Operations {
					if redefines(operation) {
						break
					}
					if uses(operation) {
						return fmt.Errorf("reference %%%d is used after its %s", ended, boundary)
					}
					if operation.IsTerminator() {
						for _, successor := range operation.Successors {
							queue = append(queue, successor.Block)
						}
					}
				}
			}
		}
	}
	return nil
}
