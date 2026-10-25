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

// A source mutable-to-shared slice call uses the existing schema-12 shared
// reborrow operation and all P15/P16 verifiers, before/after P6. This is an
// actual source build, not a handwritten or patched positive IR fixture.
// Rules: rules/memory/borrowing.md — §15.3(2);
// rules/mlir/packages/sec-mlir-dialect_package16.md — §§64–65,116,138–140.
func TestPackage16EmitSharedSliceCallReborrow(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, width := range []uint16{32, 64} {
		for _, file := range []string{"slice_shared_call_reborrow.sec", "slice_expression_call_reborrow.sec"} {
			t.Run(fmt.Sprintf("%d/%s", width, file), func(t *testing.T) {
				module := sliceEmitModule(t, file, width)
				before := semantic.Format(module)
				output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 12})
				if err != nil {
					t.Fatal(err)
				}
				for _, resolve := range []bool{false, true} {
					command := exec.Command(tool, append(sliceVerificationPipeline(resolve), "-")...)
					command.Stdin = bytes.NewReader(output)
					first, err := command.CombinedOutput()
					if err != nil {
						t.Fatalf("resolve=%t: %v\n%s", resolve, err, first)
					}
					command = exec.Command(tool, append(sliceVerificationPipeline(resolve), "-")...)
					command.Stdin = bytes.NewReader(first)
					second, err := command.CombinedOutput()
					if err != nil || !bytes.Equal(first, second) {
						t.Fatalf("shared call round-trip changed: %v\n%s", err, second)
					}
				}
				if semantic.Format(module) != before {
					t.Fatal("emission changed the source module")
				}
				// An invalid lowering must not serialize a child that reused the
				// parent's exclusive borrow identity as its shared identity.
				corrupted := false
				for _, fn := range module.Functions {
					for _, block := range fn.Blocks {
						for index := range block.Operations {
							op := &block.Operations[index]
							if op.Kind == semantic.OpSliceReborrowShared {
								op.Reference.Borrow = op.Reference.SourceBorrow
								op.Slice.Reference.Borrow = op.Reference.SourceBorrow
								corrupted = true
							}
						}
					}
				}
				if !corrupted {
					t.Fatal("fixture contains no shared reborrow")
				}
				if output, err := EmitWithOptions(module, testPlan(width), EmitOptions{DialectVersion: 12}); err == nil || output != nil || !strings.Contains(err.Error(), "requires a distinct borrow derived from its source") {
					t.Fatal("invalid shared reborrow emitted partial output", err)
				}
			})
		}
	}
}
