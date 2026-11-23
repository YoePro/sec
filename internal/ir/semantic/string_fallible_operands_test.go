package semantic

import (
	"os"
	"testing"

	"sec/internal/sema"
)

// A maximal concat evaluates each Result operand once, stops at its first
// failure, and routes operand and allocation errors to one StringError target.
// A separately protected operand leaves the outer materialization in panic mode.
// Rules: rules/foundations/operators.md — §29.3(1)–(5); MD-039 §3.
func TestMD039FallibleTextOperands(t *testing.T) {
	source, err := os.ReadFile("../../../testdata/semantic_ir/md039/fallible_operands.sec")
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
		if fn.Name != "Propagate" && fn.Name != "Recover" && fn.Name != "Separate" {
			continue
		}
		t.Run(fn.Name, func(t *testing.T) {
			blocks := map[BlockID]*Block{}
			calls := map[string]int{}
			materializations := 0
			failures := []BlockID{}
			for _, block := range fn.Blocks {
				blocks[block.ID] = block
				for _, op := range block.Operations {
					if op.Kind == OpDirectCall {
						calls[names[op.Callee]]++
					}
					if op.Kind == OpStringMaterialize {
						materializations++
						wantMode, wantFallible := sema.StringMaterializationTry, 2
						if fn.Name == "Separate" {
							wantMode, wantFallible = sema.StringMaterializationPanic, 0
						}
						if op.StringMaterialization.Failure != wantMode {
							t.Fatal("incorrect source-selected failure mode")
						}
						fallible := 0
						for _, segment := range op.StringMaterialization.Segments {
							if segment == sema.StringConcatFallibleText {
								fallible++
							}
						}
						if fallible != wantFallible {
							t.Fatal("lost fallible segments", fallible)
						}
					}
					if op.Kind == OpResultIsErr {
						// Result tests always branch to the failure block first.
						last := block.Operations[len(block.Operations)-1]
						if last.Kind != OpCondBranch || len(last.Successors) != 2 || last.Operands[0] != op.Results[0].ID {
							t.Fatal("missing failure branch")
						}
						failures = append(failures, last.Successors[0].Block)
					}
				}
			}
			if materializations != 1 || calls["First"] != 1 || calls["Last"] != 1 {
				t.Fatal("operands must evaluate once", calls, materializations)
			}
			wantChecks := 3 // two operands plus the materialization itself
			if fn.Name == "Separate" {
				wantChecks = 1
			} else if calls["Second"] != 1 {
				t.Fatal(calls)
			}
			if len(failures) != wantChecks {
				t.Fatal("lost independent failures", len(failures))
			}
			var common BlockID
			for index, failure := range failures {
				block := blocks[failure]
				last := block.Operations[len(block.Operations)-1]
				if fn.Name != "Separate" {
					if last.Kind != OpBranch || len(last.Successors) != 1 {
						t.Fatal("failure must join StringError routing")
					}
					if index == 0 {
						common = last.Successors[0].Block
					} else if common != last.Successors[0].Block {
						t.Fatal("operand/allocation errors have different routes")
					}
				}
				// No path from an error edge may evaluate a later operand or
				// allocate the string. Successful recovery produces fallback.
				seen := map[BlockID]bool{}
				pending := []BlockID{failure}
				for len(pending) > 0 {
					current := pending[len(pending)-1]
					pending = pending[:len(pending)-1]
					if seen[current] {
						continue
					}
					seen[current] = true
					for _, op := range blocks[current].Operations {
						if op.Kind == OpDirectCall || op.Kind == OpStringMaterialize {
							t.Fatalf("failure reaches later evaluation: %s", op.Kind)
						}
						for _, successor := range op.Successors {
							pending = append(pending, successor.Block)
						}
					}
				}
			}
		})
	}
}
