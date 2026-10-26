package secmlir

import (
	"bytes"
	"fmt"
	"os/exec"
	"sec/internal/ir/semantic"
	"sec/internal/testsupport"
	"testing"
)

// Branch-local slice borrow ends retain resolved identities through all native slice and
// reference verifiers before/after scalar layout, without changing the IR.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§55,58–62,64–67,138–141.
func TestPackage16EmitBranchSliceEnds(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			module := sliceEmitModule(t, "slice_branch_ends.sec", width)
			before := semantic.Format(module)
			output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 12})
			if err != nil {
				t.Fatal(err)
			}
			for _, resolve := range []bool{false, true} {
				first := exec.Command(tool, append(sliceVerificationPipeline(resolve), "-")...)
				first.Stdin = bytes.NewReader(output)
				canonical, err := first.CombinedOutput()
				if err != nil {
					t.Fatalf("resolve=%t: %v\n%s", resolve, err, canonical)
				}
				second := exec.Command(tool, append(sliceVerificationPipeline(resolve), "-")...)
				second.Stdin = bytes.NewReader(canonical)
				repeated, err := second.CombinedOutput()
				if err != nil || !bytes.Equal(canonical, repeated) {
					t.Fatalf("unstable round trip: %v\n%s", err, repeated)
				}
			}
			if before != semantic.Format(module) {
				t.Fatal("emission changed source facts")
			}
			if out, err := Emit(module, testPlan(width)); err == nil || out != nil {
				t.Fatal("default schema admitted slices")
			}
			corrupted := false
			for _, fn := range module.Functions {
				if fn.Name != "Mixed" {
					continue
				}
				for _, block := range fn.Blocks {
					for index := range block.Operations {
						op := &block.Operations[index]
						if op.Kind == semantic.OpReferenceEndBorrow {
							op.Reference.Lifetime++
							corrupted = true
						}
					}
				}
			}
			if !corrupted {
				t.Fatal("no path-specific ends to corrupt")
			}
			if out, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 12}); err == nil || out != nil {
				t.Fatal("wrong branch-end lifetime emitted bytes")
			}

		})
	}
}
