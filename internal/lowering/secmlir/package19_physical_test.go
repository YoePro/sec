package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"testing"

	"sec/internal/platform/target"
	"sec/internal/testsupport"
)

// TestPackage19NoPhysicalLowering (P19-30, P19 §§ 146-148, 161): a schema-15
// module with Arena creation, allocation, Reset and implicit destruction
// selects no physical representation, allocator, runtime or LLVM form; the
// Sec MLIR -> LLVM track rejects Arena operations as an unsupported lowering
// instead of inventing a descriptor, provider ABI or epoch table.
// Rules: rules/mlir/packages/sec-mlir-dialect_package19.md — §§ 146-148, 161.
func TestPackage19NoPhysicalLowering(t *testing.T) {
	source := `module main

fn Use() int32 {
    let mut buffer: byte[64]
    let mut arena := Arena.FromBuffer(ref mut buffer[..])
    let values := try arena.Alloc[int32](4u) {
        Err(_) => { return 0 }
    }
    values[0] = 1
    let first := values[0]
    arena.Reset()
    return first
}

fn main() int {
    discard Use()
    return 0
}
`
	module, err := buildPackage19TargetSource(t, source, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	definition, _ := target.Find(target.Target{OS: "linux", Arch: "amd64"})
	plan, _ := definition.ScalarPlan()
	output, err := EmitWithOptions(module, plan, EmitOptions{DialectVersion: 15})
	if err != nil {
		t.Fatal(err)
	}
	for _, physical := range []string{"llvm.", "memref", "malloc", "\"free\"", "realloc", "mmap", "gc_", "refcount", "vtable", "segment_list", "epoch_table", "unwind"} {
		if bytes.Contains(output, []byte(physical)) {
			t.Errorf("schema-15 Arena module chooses a physical representation: %q", physical)
		}
	}
	tool := testsupport.RequireSecMLIROptPath(t)
	lower := exec.Command(tool, "--sec-lower-to-llvm", "-", "-o", os.DevNull)
	lower.Stdin = bytes.NewReader(output)
	result, err := lower.CombinedOutput()
	if err == nil || !bytes.Contains(result, []byte("[unsupported-lowering]")) || !bytes.Contains(result, []byte("sec.arena.")) {
		t.Fatalf("the LLVM track did not reject Arena operations as an unsupported lowering: %v\n%s", err, result)
	}
}
