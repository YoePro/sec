package semantic

import (
	"os"
	"testing"
)

// Local slice holders use Sema's last-required-use schedule, including a
// copied shared holder, a transferred mutable holder and a child reborrow.
// The automatic ends precede the next write to the backing array. This
// covers the holder's own statement list, not general path-specific ends.
// Rules: rules/memory/borrowing.md — §8.1(9)–(14);
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§39–41,64–65,83–86.
func TestPackage16AutomaticSliceBorrowEnds(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/slice_source_automatic_ends.sec")
	if err != nil {
		t.Fatal(err)
	}
	for _, width := range []uint16{32, 64} {
		module, err := buildPackage16Source(t, string(source), width)
		if err != nil {
			t.Fatal(err)
		}
		for name, want := range map[string]int{"Shared": 1, "SharedCopy": 1, "MutableMove": 1, "Reborrow": 2, "Unused": 1} {
			function := functionNamed(t, module, name)
			ends, lastEnd, ownerWrite := 0, -1, -1
			var moved ValueID
			var ended []ValueID
			var endBlocks []BlockID
			var ownerBlock BlockID
			for _, block := range function.Blocks {
				for index, op := range block.Operations {
					switch op.Kind {
					case OpSliceMoveMutable:
						moved = op.Results[0].ID
					case OpReferenceEndBorrow:
						ends++
						lastEnd = index
						ended = append(ended, op.Operands[0])
						endBlocks = append(endBlocks, block.ID)
						if op.Reference == nil || op.Reference.Borrow == 0 || op.Reference.Lifetime == 0 {
							t.Fatalf("%s lost resolved borrow/lifetime at its end", name)
						}
					case OpStorageStore:
						ownerWrite = index // The final write targets the backing array.
						ownerBlock = block.ID
					}
				}
			}
			if ends != want || ownerWrite < 0 || lastEnd >= ownerWrite {
				t.Fatalf("%s/%d: ends=%d, want %d before owner write:\n%s", name, width, ends, want, Format(module))
			}
			for _, block := range endBlocks {
				if block != ownerBlock {
					t.Fatalf("fixture must exercise ends and owner write in the same continuation block")
				}
			}
			if name == "MutableMove" && (moved == 0 || len(ended) != 1 || ended[0] != moved) {
				t.Fatalf("move must end the surviving holder, not its unavailable source: %v", ended)
			}
		}
	}
}
