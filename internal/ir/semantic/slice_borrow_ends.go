package semantic

// Slice automatic end candidates are Sema's published positions, not a second
// liveness analysis. This pass tracks only whether those ends have executed,
// resetting at the SSA definition on loop reentry. It removes redundant
// candidates and places a mixed join's end on its still-live incoming edges.
// Rules: rules/memory/borrowing.md — §8.1(9)–(14);
// rules/corrections/applied/md065-path-exact-borrow-end-correction-20261021.md — §§6(1)–(5),7(1)–(2).
func (fb *functionBuilder) normalizeSliceBorrowEnds() error {
	values := map[ValueID]bool{}
	for _, block := range fb.fn.Blocks {
		for _, op := range block.Operations {
			if op.Kind == OpReferenceEndBorrow && op.Synthesized {
				values[op.Operands[0]] = true
			}
		}
	}
	// Visit SSA definitions in their deterministic source/build order.
	var ordered []ValueID
	for _, parameter := range fb.fn.Parameters {
		if values[parameter.Value.ID] {
			ordered = append(ordered, parameter.Value.ID)
		}
	}
	for _, block := range fb.fn.Blocks {
		for _, parameter := range block.Parameters {
			if values[parameter.ID] {
				ordered = append(ordered, parameter.ID)
			}
		}
		for _, op := range block.Operations {
			for _, result := range op.Results {
				if values[result.ID] {
					ordered = append(ordered, result.ID)
				}
			}
		}
	}
	for _, value := range ordered {
		if err := fb.normalizeSliceHolderEnds(value); err != nil {
			return err
		}
	}
	return nil
}

const (
	sliceEndAbsent uint8 = 1 << iota
	sliceEndLive
	sliceEndEnded
)

// sliceEndTransfer tracks one holder through definitions and explicit or
// automatic ends. It does not derive an endpoint from uses or origin aliases.
// Rules: rules/corrections/applied/md065-path-exact-borrow-end-correction-20261021.md — §§6–7;
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–41,63.
func sliceEndTransfer(state uint8, op Operation, value ValueID) uint8 {
	for _, result := range op.Results {
		if result.ID == value {
			return sliceEndLive
		}
	}
	if (op.Kind == OpReferenceEndBorrow || op.Kind == OpSliceMoveMutable) && len(op.Operands) > 0 && op.Operands[0] == value {
		return sliceEndEnded
	}
	return state
}

// normalizeSliceHolderEnds preserves Sema's candidate boundary while removing
// already-ended paths. A mixed join splits uniformly known incoming
// edges, including through transparent unconditional forwarding blocks, before
// any real instruction. Other layouts fail closed;
// no new semantic end is guessed from a textual last use or lexical scope.
// Rules: rules/memory/borrowing.md — §8.1(11)–(14);
// rules/corrections/applied/md065-path-exact-borrow-end-correction-20261021.md — §§6(2)–(5),7.
func (fb *functionBuilder) normalizeSliceHolderEnds(value ValueID) error {
	incoming, outgoing := map[BlockID]uint8{}, map[BlockID]uint8{}
	seed := sliceEndAbsent
	for _, parameter := range fb.fn.Parameters {
		if parameter.Value.ID == value {
			seed = sliceEndLive
		}
	}
	incoming[fb.fn.Entry] = seed
	changed := true
	for changed {
		changed = false
		for _, block := range fb.fn.Blocks {
			state := incoming[block.ID]
			if state == 0 {
				continue
			}
			for _, parameter := range block.Parameters {
				if parameter.ID == value {
					state = sliceEndLive
				}
			}
			for _, op := range block.Operations {
				state = sliceEndTransfer(state, op, value)
			}
			if outgoing[block.ID] != state {
				outgoing[block.ID] = state
				changed = true
			}
			if len(block.Operations) == 0 {
				continue
			}
			for _, edge := range block.Operations[len(block.Operations)-1].Successors {
				combined := incoming[edge.Block] | state
				if combined != incoming[edge.Block] {
					incoming[edge.Block] = combined
					changed = true
				}
			}
		}
	}
	// Snapshot the original blocks: edge splitting appends new blocks.
	blocks := append([]*Block(nil), fb.fn.Blocks...)
	for _, block := range blocks {
		state := incoming[block.ID]
		if state != 0 {
			for _, parameter := range block.Parameters {
				if parameter.ID == value {
					state = sliceEndLive
				}
			}
		}
		var retained []Operation
		prefixEnds := true
		for _, op := range block.Operations {
			candidate := op.Kind == OpReferenceEndBorrow && op.Synthesized && op.Operands[0] == value
			if !candidate {
				retained = append(retained, op)
				state = sliceEndTransfer(state, op, value)
				if op.Kind != OpReferenceEndBorrow {
					prefixEnds = false
				}
				continue
			}
			switch state {
			case sliceEndEnded:
				// Every incoming path already executed this Sema end.
			case sliceEndLive, 0:
				retained = append(retained, op)
			case sliceEndLive | sliceEndEnded:
				if !prefixEnds {
					return fb.unsupportedSliceEndJoin(op)
				}
				edges, ok := sliceLiveEndEdges(block.ID, blocks, outgoing)
				if !ok {
					return fb.unsupportedSliceEndJoin(op)
				}
				for _, incoming := range edges {
					terminator := &incoming.block.Operations[len(incoming.block.Operations)-1]
					edge := terminator.Successors[incoming.index]
					split := fb.newBlock()
					split.Operations = []Operation{op, {Kind: OpBranch, Successors: []BranchTarget{edge}, Location: op.Location, Synthesized: true}}
					terminator.Successors[incoming.index] = BranchTarget{Block: split.ID}
				}
			default:
				return fb.unsupportedSliceEndJoin(op)
			}
			state = sliceEndEnded
		}
		block.Operations = retained
	}
	return nil
}

// unsupportedSliceEndJoin names the explicit boundary when the published
// endpoint cannot be represented without a more detailed Sema edge schedule.
// Rules: rules/corrections/applied/md065-path-exact-borrow-end-correction-20261021.md — §6(5).
func (fb *functionBuilder) unsupportedSliceEndJoin(op Operation) error {
	return &UnsupportedFeatureError{Feature: "slice borrow end at a join requiring a resolved edge schedule (P16-36 / MD-065)", Package: fb.owner.maxPackage, Location: op.Location}
}
