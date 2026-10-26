package semantic

import (
	"os"
	"testing"
)

// Sema-published ends of outer slice holders apply to individual branches.
// Each reachable path ends once before owner mutation; iteration-local SSA
// definitions restart the obligation. A mixed join ends only its live edge.
// Rules: rules/memory/borrowing.md — §8.1(9)–(14), §17(3)–(6);
// rules/corrections/applied/md065-path-exact-borrow-end-correction-20261021.md — §§6–8.
func TestPackage16BranchSliceEnds(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/slice_branch_ends.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		module, err := buildPackage16Source(t, string(source), width)
		if err != nil {
			t.Fatal(err)
		}
		if err := Verify(module); err != nil {
			t.Fatal(err)
		}
		repeated, err := buildPackage16Source(t, string(source), width)
		if err != nil || Format(module) != Format(repeated) {
			t.Fatalf("unstable endpoint placement: %v", err)
		}
		for name, want := range map[string]int{"Both": 2, "Mixed": 2, "Mutable": 2, "Terminal": 2, "Iteration": 1, "Nested": 3, "Early": 2, "NestedEarly": 2, "EarlyUnused": 2, "MutableEarly": 2, "Transparent": 3, "DeepTransparent": 4} {
			fn := functionNamed(t, module, name)
			blocks := map[BlockID]*Block{}
			held := ValueID(0)
			ends := 0
			var owner StorageID
			for _, block := range fn.Blocks {
				blocks[block.ID] = block
				for _, op := range block.Operations {
					if op.Kind == OpSliceBorrowShared || op.Kind == OpSliceBorrowMutable {
						held = op.Results[0].ID
						// The backing Place is derived from the fixed-array storage, and
						// this fixture has a single fixed-array storage per function.
					}
					if op.Kind == OpReferenceEndBorrow {
						ends++
						if op.Reference == nil || op.Reference.Borrow == 0 || op.Reference.Lifetime == 0 {
							t.Fatal("lost endpoint identity")
						}
					}
				}
			}
			for _, storage := range fn.Storages {
				typ, _ := module.Types.Lookup(storage.Type)
				if typ.Kind == TypeArray {
					owner = storage.ID
				}
			}
			if held == 0 || owner == 0 || ends != want {
				t.Fatalf("%s: holder=%d owner=%d ends=%d want=%d", name, held, owner, ends, want)
			}
			type state struct {
				block  BlockID
				active bool
				ends   int
			}
			queue := []state{{block: fn.Entry}}
			seen := map[state]bool{}
			for len(queue) > 0 {
				current := queue[0]
				queue = queue[1:]
				if seen[current] {
					continue
				}
				seen[current] = true
				for _, op := range blocks[current.block].Operations {
					for _, result := range op.Results {
						if result.ID == held {
							current.active = true
							current.ends = 0
						}
					}
					if op.Kind == OpReferenceEndBorrow && op.Operands[0] == held {
						current.ends++
						if !current.active || current.ends != 1 {
							t.Fatalf("%s: duplicate/premature end on path", name)
						}
					}
					if op.Kind == OpStorageStore && op.Storage == owner && current.active && current.ends != 1 {
						t.Fatalf("%s: owner mutation before resolved end", name)
					}
				}
				last := blocks[current.block].Operations[len(blocks[current.block].Operations)-1]
				for _, edge := range last.Successors {
					queue = append(queue, state{edge.Block, current.active, current.ends})
				}
			}
		}
	}
}

// A required post-join use prevents either branch from releasing authority.
// Rules: rules/memory/borrowing.md — §8.1(7),(11), §17(4).
func TestPackage16BranchSliceRetainedUse(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/sema/slice_branch_ends_invalid.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		module, err := buildPackage16Source(t, string(source), width)
		if err == nil || module != nil {
			t.Fatal("future use allowed early borrow termination")
		}
	}
}
