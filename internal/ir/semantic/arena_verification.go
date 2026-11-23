package semantic

import "fmt"

// verifyArenaDomains checks freshness of static creation identities, including
// independently represented functions. It does not derive domains for owners
// received as parameters or create another domain/epoch allocator.
// Rules: rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md — §§3,6–7,26.
func verifyArenaDomains(module *Module) error {
	seen := map[uint32]bool{}
	for _, fn := range module.Functions {
		if fn == nil {
			continue
		}
		for _, block := range fn.Blocks {
			if block == nil {
				continue
			}
			for _, op := range block.Operations {
				if op.Kind != OpArenaCreateBorrowed && op.Kind != OpArenaCreateOwnedFixed && op.Kind != OpArenaCreateGrowable && op.Kind != OpArenaCreateDefault {
					continue
				}
				if op.Backing == nil || op.Backing.Domain == 0 {
					return fmt.Errorf("%s lacks its fresh domain", op.Kind)
				}
				if seen[op.Backing.Domain] {
					return fmt.Errorf("Arena creation reuses domain #%d", op.Backing.Domain)
				}
				seen[op.Backing.Domain] = true
			}
		}
	}
	return nil
}

// verifyArenaTransfers rejects any reachable use after transferring the backing
// borrow or consuming an Arena. A repeated dynamic execution of the SSA
// definition starts a new value, so loop-local creation is not a double release.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§89,95–96;
// rules/mlir/semantic-ir/sec_semantic_ir_arena_v19.md — §§2,12–13.
func verifyArenaTransfers(module *Module, fn *Function, blocks map[BlockID]*Block) error {
	sources := map[ValueID]*ReferenceMetadata{}
	bounds := map[ValueID]*SliceFacts{}
	for _, p := range fn.Parameters {
		sources[p.Value.ID] = p.Reference
	}
	for _, block := range fn.Blocks {
		for _, op := range block.Operations {
			for _, result := range op.Results {
				sources[result.ID] = op.Reference
				bounds[result.ID] = op.Slice
			}
		}
	}
	resolveForwardedSliceFacts(module.Types, fn, blocks, sources)

	type position struct {
		block BlockID
		index int
	}
	for _, block := range fn.Blocks {
		for index, consume := range block.Operations {
			var source *ReferenceMetadata
			if consume.Kind == OpArenaCreateBorrowed {
				source = sources[consume.Operands[0]]
			}
			var rangeFacts *SliceFacts
			if consume.Kind == OpArenaCreateBorrowed {
				rangeFacts = bounds[consume.Operands[0]]
			}
			if err := verifyArenaParent(consume, source, rangeFacts); err != nil {
				return err
			}
			if consume.Kind != OpArenaCreateBorrowed && consume.Kind != OpArenaRelease && consume.Kind != OpArenaDestroy {
				continue
			}
			value := consume.Operands[0]
			if consume.Kind == OpArenaCreateBorrowed {
				reference := sources[value]
				if reference == nil || reference.Borrow != consume.Arena.RetainedBorrow {
					return fmt.Errorf("arena.create-borrowed must retain the transferred backing borrow")
				}
			}

			pending := []position{{block.ID, index + 1}}
			seen := map[position]bool{}
			for len(pending) > 0 {
				at := pending[len(pending)-1]
				pending = pending[:len(pending)-1]
				if seen[at] {
					continue
				}
				seen[at] = true
				current := blocks[at.block]
				for i := at.index; i < len(current.Operations); i++ {
					op := current.Operations[i]
					renewed := false
					for _, result := range op.Results {
						renewed = renewed || result.ID == value
					}
					if renewed {
						break
					}
					for _, operand := range operationOperands(op) {
						if operand == value {
							return fmt.Errorf("%%%d used by %s after %s transferred it", value, op.Kind, consume.Kind)
						}
					}
					for _, successor := range op.Successors {
						pending = append(pending, position{successor.Block, 0})
					}
				}
			}
		}
	}
	return nil
}
