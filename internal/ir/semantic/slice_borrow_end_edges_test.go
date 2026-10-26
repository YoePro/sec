package semantic

import (
	"reflect"
	"testing"
)

// An endpoint may traverse branch-only joins, but cannot move across an
// instruction, parameter definition or cyclic ambiguity. A rejected plan must
// not mutate any CFG edge, even after discovering a valid live predecessor.
// Rules: rules/corrections/applied/md065-path-exact-borrow-end-correction-20261021.md — §6(2)–(5);
// rules/memory/borrowing.md — §8.1(11)–(14).
func TestSliceEndJoinPlanningPreservesBoundary(t *testing.T) {
	for _, boundary := range []string{"transparent", "instruction", "parameter", "cycle"} {
		t.Run(boundary, func(t *testing.T) {
			blocks := []*Block{
				{ID: 1, Operations: []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 3, Arguments: []ValueID{10}}}}}},
				{ID: 2, Operations: []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 3, Arguments: []ValueID{11}}}}}},
				{ID: 3, Operations: []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 4, Arguments: []ValueID{12}}}}}},
				{ID: 4},
			}
			outgoing := map[BlockID]uint8{1: sliceEndLive, 2: sliceEndEnded, 3: sliceEndLive | sliceEndEnded}
			switch boundary {
			case "instruction":
				blocks[2].Operations = append([]Operation{{Kind: OpStorageLoad}}, blocks[2].Operations...)
			case "parameter":
				blocks[2].Parameters = []Value{{ID: 12}}
			case "cycle":
				// The mixed forwarding chain closes 4 -> 5 -> 3 -> 4.
				blocks[3].Operations = []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 5}}}}
				blocks = append(blocks, &Block{ID: 5, Operations: []Operation{{Kind: OpBranch, Successors: []BranchTarget{{Block: 3}}}}})
				outgoing[4], outgoing[5] = sliceEndLive|sliceEndEnded, sliceEndLive|sliceEndEnded
			}
			before := make([][]Operation, len(blocks))
			for i, block := range blocks {
				before[i] = append([]Operation(nil), block.Operations...)
				for j := range before[i] {
					before[i][j].Successors = append([]BranchTarget(nil), block.Operations[j].Successors...)
				}
			}
			edges, ok := sliceLiveEndEdges(4, blocks, outgoing)
			if boundary == "transparent" {
				if !ok || len(edges) != 1 || edges[0].block.ID != 1 || edges[0].index != 0 {
					t.Fatalf("wrong live path: %v known=%v", edges, ok)
				}
				if !reflect.DeepEqual(edges[0].block.Operations[0].Successors[0].Arguments, []ValueID{10}) {
					t.Fatal("changed successor arguments")
				}
			} else if ok || edges != nil {
				t.Fatal("endpoint crossed a semantic boundary")
			}
			for i, block := range blocks {
				if !reflect.DeepEqual(before[i], block.Operations) {
					t.Fatal("planning mutated the CFG")
				}
			}
		})
	}
}
