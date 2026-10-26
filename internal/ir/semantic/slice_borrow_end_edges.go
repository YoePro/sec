package semantic

// sliceEndEdge identifies an incoming successor slot without changing its
// operand tuple or parameter types when a resolved endpoint is split onto it.
type sliceEndEdge struct {
	block *Block
	index int
}

// sliceLiveEndEdges plans a Sema-published join endpoint on only still-live
// incoming paths. It may cross a parameter-free block containing solely an
// unconditional branch: no instruction can observe an earlier/later end.
// Mixed paths through real operations, parameter definitions or cycles remain
// unsupported. Planning completes before any CFG mutation; shared subgraphs
// and duplicate successor slots are handled deterministically.
// Rules: rules/memory/borrowing.md — §8.1(11)–(14);
// rules/corrections/applied/md065-path-exact-borrow-end-correction-20261021.md — §§6(2)–(5),7(1).
func sliceLiveEndEdges(target BlockID, blocks []*Block, outgoing map[BlockID]uint8) ([]sliceEndEdge, bool) {
	var edges []sliceEndEdge
	visiting, finished := map[BlockID]bool{}, map[BlockID]bool{}
	var collect func(BlockID) bool
	collect = func(target BlockID) bool {
		if visiting[target] {
			return false
		}
		if finished[target] {
			return true
		}
		visiting[target] = true
		for _, predecessor := range blocks {
			if len(predecessor.Operations) == 0 {
				continue
			}
			terminator := predecessor.Operations[len(predecessor.Operations)-1]
			for index, edge := range terminator.Successors {
				if edge.Block != target {
					continue
				}
				switch outgoing[predecessor.ID] {
				case sliceEndEnded, 0:
				case sliceEndLive:
					edges = append(edges, sliceEndEdge{predecessor, index})
				case sliceEndLive | sliceEndEnded:
					if len(predecessor.Parameters) != 0 || len(predecessor.Operations) != 1 || terminator.Kind != OpBranch || len(terminator.Successors) != 1 || !collect(predecessor.ID) {
						return false
					}
				default:
					return false
				}
			}
		}
		delete(visiting, target)
		finished[target] = true
		return true
	}
	if !collect(target) {
		return nil, false
	}
	return edges, true
}
