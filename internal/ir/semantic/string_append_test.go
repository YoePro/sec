package semantic

import (
	"os"
	"strings"
	"testing"
)

// Every append reads the destination before RHS evaluation and commits exactly
// once on success. No failure edge may reach that commit, including an operand
// failure and the allocation failure; handled failures preserve the old value.
// Rules: rules/foundations/operators.md — §29.9(1)–(4); MD-039 §§3–4.
func TestMD039TransactionalStringAppend(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/md039/append.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(source), 19)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatalf("%v\n%s", err, Format(module))
	}
	names := map[FunctionID]string{}
	for _, fn := range module.Functions {
		names[fn.ID] = fn.Name
	}
	for _, fn := range module.Functions {
		if fn.Name == "Text" || fn.Name == "Part" {
			continue
		}
		t.Run(fn.Name, func(t *testing.T) {
			blocks := map[BlockID]*Block{}
			failures := []BlockID{}
			reads, writes, materializations, calls := 0, 0, 0, 0
			var commitBlock BlockID
			var oldValue, destination ValueID
			var staticDestination StaticID
			for _, block := range fn.Blocks {
				blocks[block.ID] = block
				for _, op := range block.Operations {
					switch op.Kind {
					case OpPlaceRead, OpStaticLoad:
						// The destination read occurs before the first operand
						// call. A later read returns the post-append value.
						if calls == 0 {
							reads++
							oldValue = op.Results[0].ID
							if op.Kind == OpPlaceRead {
								destination = op.Operands[0]
							} else {
								staticDestination = op.Static
							}
						}
					case OpDirectCall:
						calls++
						if reads != 1 {
							t.Fatal("RHS ran before the single destination read", reads)
						}
					case OpStringMaterialize:
						materializations++
						if op.Operands[1] != oldValue {
							t.Fatal("append lost its original destination value")
						}
					case OpPlaceWrite, OpStaticStore:
						writes++
						commitBlock = block.ID
						if op.Kind == OpPlaceWrite && op.Operands[0] != destination {
							t.Fatal("append commits to a different Place")
						}
						if op.Kind == OpStaticStore && op.Static != staticDestination {
							t.Fatal("append commits to different static storage")
						}
					case OpResultIsErr:
						last := block.Operations[len(block.Operations)-1]
						if last.Kind != OpCondBranch {
							t.Fatal("unchecked Result")
						}
						failures = append(failures, last.Successors[0].Block)
					}
				}
			}
			if reads != 1 || writes != 1 || materializations != 1 {
				t.Fatal("append must prepare/commit once", reads, writes, materializations)
			}
			if fn.Name == "Fused" && calls != 2 {
				t.Fatal("fused RHS duplicated or lost calls", calls)
			}
			for _, failure := range failures {
				seen := map[BlockID]bool{}
				pending := []BlockID{failure}
				for len(pending) > 0 {
					current := pending[len(pending)-1]
					pending = pending[:len(pending)-1]
					if seen[current] {
						continue
					}
					seen[current] = true
					if current == commitBlock {
						t.Fatal("failure can commit the append")
					}
					for _, op := range blocks[current].Operations {
						for _, successor := range op.Successors {
							pending = append(pending, successor.Block)
						}
					}
				}
			}
		})
	}
}

// Runtime formatting is still a named capability boundary. Append support
// must not erase an interpolation's canonical formatting plan by flattening it.
// Rules: rules/foundations/operators.md — §§29.4,29.9; MD-039 §1(4).
func TestMD039AppendPreservesInterpolationBoundary(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/md039/append_interpolation_boundary.sec")
	if err != nil {
		t.Fatal(err)
	}
	module, err := analyzedModule(t, string(source), 19)
	if module != nil || err == nil || !strings.Contains(err.Error(), "string append over runtime interpolation") {
		t.Fatalf("lost formatting boundary: %v", err)
	}
}
