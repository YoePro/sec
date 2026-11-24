package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/platform/target"
	"sec/internal/testsupport"
)

// P19-33 (P19 § 151; arena.md § 31; P19 § 35; v15 §§ 9, 14): the allocation
// atomicity matrix through Semantic IR and schema-15 Sec MLIR on a fixed
// borrowed (FromBuffer) Arena.
//   - fixed success: the candidate is published as Ok on the success edge.
//   - fixed capacity failure: the failure edge builds Err(AllocationError)
//     and carries no candidate; a local handler continues with the Arena.
//   - failure leaves the state version and epoch unchanged: the failure
//     block performs no Arena mutation or store, New/Alloc preserve the
//     epoch, and the state version advances only on the success edge.
//   - failure leaves old allocations valid: the earlier slice is read after
//     the handled failure with its unchanged domain and epoch facts.
//   - no partial ref/slice published: every candidate use is dominated by
//     the success edge; a candidate moved across the failure edge or used
//     after a join is rejected by the Semantic IR verifier (and by
//     --sec-verify-arenas, mlir/test/Analysis/invalid-arena-publication.mlir).
//
// The growable rows (segment success, provider failure) need the
// sec.system-heap.v1 provider (MD-151, target_profiles.md § 41A); growable
// allocation is a named lowering boundary until then (governance remaining
// item).
func TestPackage19AllocationAtomicityMatrix(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

type Point struct {
    x: int32,
    y: int64,
}

fn Fixed() int32 {
    let mut buffer: byte[16]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    let first := try arena.Alloc[int32](2u) {
        Err(_) => { return 1 }
    }
    first[0] = 4
    let mut pending := true
    while pending {
        let second := try arena.Alloc[int32](3u) {
            Err(_) => { break }
        }
        second[0] = 5
        pending = false
    }
    let kept := first[0]
    let point := try arena.New[Point]() {
        Err(_) => { return kept }
    }
    point.x = kept
    return point.x
}

fn main() int {
    discard Fixed()
    return 0
}
`
	module, err := buildPackage19TargetSource(t, source, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	var fixed *semantic.Function
	for _, function := range module.Functions {
		if function.Name == "Fixed" {
			fixed = function
		}
	}
	if fixed == nil {
		t.Fatal("no Fixed function")
	}
	blocks := map[semantic.BlockID]*semantic.Block{}
	for _, block := range fixed.Blocks {
		blocks[block.ID] = block
	}
	checked := 0
	for _, block := range fixed.Blocks {
		for index, op := range block.Operations {
			if op.Kind != semantic.OpArenaAlloc && op.Kind != semantic.OpArenaNew {
				continue
			}
			checked++
			if op.Arena == nil || !op.Arena.FailureAtomic || op.Arena.EpochBefore != op.Arena.EpochAfter || op.Arena.StateAfter != op.Arena.StateBefore+1 {
				t.Fatalf("%s: atomic, epoch-preserving single state step required: %+v", op.Kind, op.Arena)
			}
			branch := block.Operations[index+1]
			if branch.Kind != semantic.OpCondBranch || branch.Operands[0] != op.Results[1].ID {
				t.Fatalf("%s must branch on its failed flag: %v", op.Kind, branch)
			}
			failure, success := blocks[branch.Successors[0].Block], blocks[branch.Successors[1].Block]
			if len(branch.Successors[0].Arguments) != 1 || branch.Successors[0].Arguments[0] != op.Results[2].ID {
				t.Fatalf("%s: the failure edge carries only the AllocationError: %v", op.Kind, branch.Successors[0])
			}
			sawErr := false
			for _, failed := range failure.Operations {
				switch failed.Kind {
				case semantic.OpResultErr:
					sawErr = true
				case semantic.OpArenaNew, semantic.OpArenaAlloc, semantic.OpArenaReset, semantic.OpArenaRelease, semantic.OpPlaceWrite, semantic.OpStorageStore, semantic.OpStorageInit:
					t.Fatalf("%s: the failure edge mutates state: %v", op.Kind, failed)
				}
				for _, operand := range failed.Operands {
					if operand == op.Results[0].ID {
						t.Fatalf("%s: the failure edge reads the candidate: %v", op.Kind, failed)
					}
				}
			}
			if !sawErr {
				t.Fatalf("%s: the failure edge must build Err(AllocationError)", op.Kind)
			}
			if success.Operations[0].Kind != semantic.OpResultOk || success.Operations[0].Operands[0] != op.Results[0].ID {
				t.Fatalf("%s: the candidate is published as Ok on the success edge: %v", op.Kind, success.Operations[0])
			}
		}
	}
	if checked != 3 {
		t.Fatalf("expected three checked allocations, found %d", checked)
	}

	definition, _ := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	plan, _ := definition.ScalarPlan()
	output, err := EmitWithOptions(module, plan, EmitOptions{DialectVersion: 15})
	if err != nil {
		t.Fatal(err)
	}
	text := package19FunctionText(t, string(output), "Fixed")
	// The earlier slice keeps its domain and epoch after the handled failure:
	// one Arena reference fact set for every slice access in the function.
	reference := `validity_policy = "dynamic-epoch", invalidation_domain = `
	facts := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		if start := strings.Index(line, reference); start >= 0 && strings.Contains(line, `"sec.slice.`) {
			end := strings.Index(line[start:], ", address_stability")
			facts[line[start:start+end]] = true
		}
	}
	if len(facts) != 1 {
		t.Fatalf("slice accesses must keep one Arena domain and epoch across the failure: %v\n%s", facts, text)
	}
	verify := exec.Command(tool, "--sec-verify-arenas", "--sec-verify-allocation-contexts", "--sec-verify-places", "--sec-verify-reference-guards",
		"--sec-verify-storage-transitions", "--sec-verify-cleanups", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s\n%s", err, result, text)
	}

	// No partial publication: moving the candidate across the failure edge
	// is rejected by the Semantic IR verifier.
	for _, block := range fixed.Blocks {
		for index, op := range block.Operations {
			if op.Kind == semantic.OpArenaAlloc {
				edge := &block.Operations[index+1].Successors[0]
				edge.Arguments = append(edge.Arguments, op.Results[0].ID)
				leaked := op.Results[0]
				leaked.ID = 1 << 30
				blocks[edge.Block].Parameters = append(blocks[edge.Block].Parameters, leaked)
				if err := semantic.Verify(module); err == nil || !strings.Contains(err.Error(), "across the failure branch") {
					t.Fatalf("a candidate across the failure edge must be rejected: %v", err)
				}
				return
			}
		}
	}
}
