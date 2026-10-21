package semantic

// cfgValueOrigins follows block-argument transport to defining SSA values.
// Every incoming edge participates; rootless cycles taint downstream joins.
// This is provenance forwarding, not value equality or a new range analysis.
// Rules: rules/compiler/semantic_ir.md — §§20,66,126.23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§79–86.
func cfgValueOrigins(fn *Function, blocks map[BlockID]*Block) (map[ValueID]map[ValueID]bool, map[ValueID]bool) {
	incoming := map[ValueID][]ValueID{}
	for _, block := range fn.Blocks {
		for _, parameter := range block.Parameters {
			incoming[parameter.ID] = nil
		}
	}
	for _, block := range fn.Blocks {
		for _, operation := range block.Operations {
			for _, successor := range operation.Successors {
				for index, arg := range successor.Arguments {
					id := blocks[successor.Block].Parameters[index].ID
					incoming[id] = append(incoming[id], arg)
				}
			}
		}
	}
	roots := map[ValueID]map[ValueID]bool{}
	for id := range incoming {
		roots[id] = map[ValueID]bool{}
	}
	changed := true
	for changed {
		changed = false
		for id, args := range incoming {
			for _, arg := range args {
				candidates, forwarded := roots[arg]
				if !forwarded {
					candidates = map[ValueID]bool{arg: true}
				}
				for root := range candidates {
					if !roots[id][root] {
						roots[id][root] = true
						changed = true
					}
				}
			}
		}
	}
	unknown := map[ValueID]bool{}
	for id, candidates := range roots {
		unknown[id] = len(candidates) == 0
	}
	changed = true
	for changed {
		changed = false
		for id, args := range incoming {
			for _, arg := range args {
				if unknown[arg] && !unknown[id] {
					unknown[id], changed = true, true
				}
			}
		}
	}
	return roots, unknown
}

// sliceVerificationView detaches only the CFG/operand containers needed to
// check slice proof identity through unambiguous forwarding. Distinct roots,
// even equal constants or equal reference metadata, never become one SSA value.
// Generic SSA/dominance and lifetime verification still consume original IR.
// Rules: rules/compiler/semantic_ir.md — §§20,66,126.23;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§79–86.
func sliceVerificationView(types *TypeTable, fn *Function, blocks map[BlockID]*Block) (*Function, map[BlockID]*Block, map[ValueID]ValueID) {
	roots, unknown := cfgValueOrigins(fn, blocks)
	parameterTypes := map[ValueID]TypeID{}
	for _, block := range fn.Blocks {
		for _, parameter := range block.Parameters {
			parameterTypes[parameter.ID] = parameter.Type
		}
	}
	aliases := map[ValueID]ValueID{}
	for id, candidates := range roots {
		typ, _ := types.Lookup(parameterTypes[id])
		// P15 direct-reference and Place facts retain their own producer
		// boundary. Rewriting a dereference source could otherwise bypass
		// its original dynamic-validity checks before slice construction.
		if unknown[id] || len(candidates) != 1 || typ.Kind == TypeReference || typ.Kind == TypePlace {
			continue
		}
		for root := range candidates {
			aliases[id] = root
		}
	}
	if len(aliases) == 0 {
		return fn, blocks, aliases
	}
	view := *fn
	view.Blocks = make([]*Block, 0, len(fn.Blocks))
	viewBlocks := map[BlockID]*Block{}
	for _, block := range fn.Blocks {
		copy := *block
		copy.Operations = make([]Operation, len(block.Operations))
		for i, op := range block.Operations {
			op.Operands = append([]ValueID(nil), op.Operands...)
			for j, id := range op.Operands {
				op.Operands[j] = forwardedValue(id, aliases)
			}
			op.Successors = append([]BranchTarget(nil), op.Successors...)
			for j, target := range op.Successors {
				target.Arguments = append([]ValueID(nil), target.Arguments...)
				for k, id := range target.Arguments {
					target.Arguments[k] = forwardedValue(id, aliases)
				}
				op.Successors[j] = target
			}
			op.ReferenceGuard, op.ArrayGuard = forwardedValue(op.ReferenceGuard, aliases), forwardedValue(op.ArrayGuard, aliases)
			copy.Operations[i] = op
		}
		view.Blocks = append(view.Blocks, &copy)
		viewBlocks[copy.ID] = &copy
	}
	return &view, viewBlocks, aliases
}

// forwardedValue preserves exact SSA identity unless every incoming edge is
// proven to forward one defining value. Zero remains the absent-guard marker.
// Rules: rules/compiler/semantic_ir.md — §§20,66,126.23.
func forwardedValue(id ValueID, aliases map[ValueID]ValueID) ValueID {
	if id == 0 {
		return 0
	}
	if root, known := aliases[id]; known {
		return root
	}
	return id
}
