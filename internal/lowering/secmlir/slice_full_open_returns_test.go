package secmlir

import (
	"bytes"
	"fmt"
	"os/exec"
	"sec/internal/ir/semantic"
	"sec/internal/testsupport"
	"testing"
)

// Full-open returned slices retain source facts through all native slice and
// reference verifiers before/after scalar layout, without changing the IR.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§55,58–62,64–67,138–141.
func TestPackage16EmitFullOpenReturns(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			module := sliceEmitModule(t, "slice_full_open_returns.sec", width)
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
		})
	}
}
