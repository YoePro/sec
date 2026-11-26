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

// P19-34 (P19 § 152; arena.md §§ 36–42, 105; P19 §§ 39–41, 80, 94): the
// Reset matrix through Semantic IR and schema-15 Sec MLIR. A fixed borrowed
// (FromBuffer) and a growable (MD-136 default under hosted) Arena reset with
// a checked epoch advance and a state-version step per Reset; the backing
// and its capacity are retained (the next allocation sees the same initial
// capacity and the retained loan); the cursor resets, so the demand summary
// is the peak of the Reset windows rather than their sum; nothing zeroes the
// bytes between Reset and the next allocation; and the epoch exists only as
// logical attributes, never as a runtime SSA value. The diagnostic rows (old
// ref/slice cannot cross, nested child and defer dependencies block) are
// covered by the Sema tests listed in package19-todo.md under P19-34.
func TestPackage19ResetMatrix(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

fn Fixed() Result[int32, AllocationError] {
    let mut buffer: byte[256]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    let first := try arena.Alloc[int32](50u)
    first[0] = 1
    arena.Reset()
    let second := try arena.Alloc[int32](50u)
    second[0] = 2
    let value := second[0]
    arena.Reset()
    return Ok(value)
}

fn Growable() void {
    let mut arena: Arena
    arena.Reset()
    arena.Reset()
}

fn main() int {
    return 0
}
`
	module, err := buildPackage19TargetSource(t, source, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	resets := map[string][]semantic.Operation{}
	for _, function := range module.Functions {
		var allocs []semantic.Operation
		for _, block := range function.Blocks {
			afterReset := false
			for _, op := range block.Operations {
				switch op.Kind {
				case semantic.OpArenaReset:
					resets[function.Name] = append(resets[function.Name], op)
					if len(op.Results) != 0 {
						t.Fatalf("%s: Reset publishes a runtime value %v", function.Name, op)
					}
					afterReset = true
				case semantic.OpArenaAlloc, semantic.OpArenaNew:
					allocs = append(allocs, op)
					afterReset = false
				case semantic.OpPlaceWrite, semantic.OpStorageStore, semantic.OpStorageInit:
					// § 40: no zeroing or other write between Reset and the
					// next allocation in the same block.
					if afterReset {
						t.Fatalf("%s: store after Reset before any allocation: %v", function.Name, op)
					}
				}
			}
		}
		if function.Name == "Fixed" {
			if len(allocs) != 2 || allocs[0].Arena == nil || allocs[1].Arena == nil {
				t.Fatalf("Fixed allocations: %v", allocs)
			}
			first, second := allocs[0].Arena, allocs[1].Arena
			if first.EpochBefore != 1 || second.EpochBefore != 2 || first.CapacityPlan == "" || first.CapacityPlan != second.CapacityPlan || first.BackingKind != "borrowed" || second.BackingKind != "borrowed" || first.RetainedBorrow == 0 || first.RetainedBorrow != second.RetainedBorrow {
				t.Fatalf("capacity or epoch not retained across Reset: %+v / %+v", first, second)
			}
		}
	}
	for name, want := range map[string]struct{ backing, growth string }{
		"Fixed":    {"borrowed", "fixed"},
		"Growable": {"none", "growable-nonrelocating"},
	} {
		ops := resets[name]
		if len(ops) != 2 {
			t.Fatalf("%s: want two Resets, got %v", name, ops)
		}
		for index, op := range ops {
			facts := op.Arena
			if facts == nil || op.Backing == nil || facts.BackingKind != want.backing || facts.GrowthKind != want.growth || facts.EpochAfter != facts.EpochBefore+1 || facts.StateAfter != facts.StateBefore+1 || facts.DependencyProof == "" || op.Backing.Transition != "advance-epoch" {
				t.Fatalf("%s Reset %d: %+v %+v", name, index, facts, op.Backing)
			}
			if index == 1 && (facts.EpochBefore != ops[0].Arena.EpochAfter || facts.StateBefore < ops[0].Arena.StateAfter) {
				t.Fatalf("%s: second Reset does not continue the chain: %+v after %+v", name, facts, ops[0].Arena)
			}
		}
	}

	definition, _ := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	plan, _ := definition.ScalarPlan()
	output, err := EmitWithOptions(module, plan, EmitOptions{DialectVersion: 15})
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, `"sec.arena.reset"`) && strings.Contains(strings.SplitN(line, `"sec.arena.reset"`, 2)[0], "=") {
			t.Fatalf("sec.arena.reset has a result: %s", line)
		}
	}
	for _, want := range []string{
		`epoch_before = 1 : i32, epoch_after = 2 : i32`, `epoch_before = 2 : i32, epoch_after = 3 : i32`,
		`growth_kind = "growable-nonrelocating"`,
		// Cursor reset (§ 80): two 50 x int32 windows peak at 200 + 3
		// alignment padding bytes, not their 406-byte sum.
		`sec.arena_demand_summary = [{bytes = "203", capacity_bytes = "256"`,
		// MD-140 (v15 § 11): the checked 64-bit advance and its reason.
		`epoch_advance = "checked", epoch_width = 64 : i32, exhaustion_reason = "panic.arena-epoch-exhaustion"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("schema-15 module lacks %q:\n%s", want, text)
		}
	}
	verify := exec.Command(tool, "--sec-verify-arenas", "--sec-verify-allocation-contexts", "--sec-verify-places", "--sec-verify-reference-guards",
		"--sec-verify-cleanups", "--sec-verify-destruction-plans", "--sec-verify-ownership", "-", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s\n%s", err, result, text)
	}
}
