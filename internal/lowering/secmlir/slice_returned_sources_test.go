package secmlir

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/ir/semantic"
	"sec/internal/testsupport"
)

// Returned slice sources are built from unmodified Sema plans on both target
// widths, pass all four P15/P16 verifiers before/after scalar layout, and retain
// their provenance through deterministic native round trips.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§17,43–49,64–67,138–141.
func TestPackage16EmitReturnedSliceSources(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			module := sliceEmitModule(t, "slice_returned_expression_sources.sec", width)
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
					t.Fatalf("unstable native round trip: %v\n%s", err, repeated)
				}
			}
			if before != semantic.Format(module) {
				t.Fatal("emission changed source facts")
			}
			if out, err := Emit(module, testPlan(width)); err == nil || out != nil {
				t.Fatal("default schema admitted slices")
			}
			// Corrupt the relationship between a call result and its reborrow.
			// Validity of the earlier functions cannot permit partial output.
			changed := false
			for _, fn := range module.Functions {
				if fn.Name != "Shared" {
					continue
				}
				for _, block := range fn.Blocks {
					for index := range block.Operations {
						op := &block.Operations[index]
						if op.Kind != semantic.OpSliceReborrowShared {
							continue
						}
						op.Reference.SourceBorrow = 0
						op.Slice.Reference.SourceBorrow = 0
						changed = true
					}
				}
			}
			if !changed {
				t.Fatal("fixture has no call-result reborrow")
			}
			if out, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 12}); err == nil || out != nil || !strings.Contains(err.Error(), "requires a distinct borrow derived from its source") {
				t.Fatalf("invalid call-source provenance emitted bytes: %v", err)
			}
		})
	}
}
