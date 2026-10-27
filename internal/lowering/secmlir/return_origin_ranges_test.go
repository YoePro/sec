package secmlir

import (
	"bytes"
	"fmt"
	"os/exec"
	"sec/internal/ir/semantic"
	"sec/internal/testsupport"
	"testing"
)

// MD-115 returned ranges retain exact/unknown facts through native slice and
// reference verifiers before/after scalar layout, without changing the IR.
// Rules: rules/mlir/packages/sec-mlir-dialect_package16.md — §§55,58–62,64–67,138–141.
func TestPackage16EmitRangeReturns(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	for _, width := range []uint16{32, 64} {
		t.Run(fmt.Sprint(width), func(t *testing.T) {
			module := sliceEmitModule(t, "slice_range_returns.sec", width)
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

// MD-115 §§7,10: declared exact/unknown text cannot repair contradictory SSA,
// hide a narrower origin, invent a parameter, or bypass schema selection.
func TestPackage16RejectsForgedRangeReturns(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	module := sliceEmitModule(t, "slice_range_returns.sec", 64)
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{DialectVersion: 12})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, old, replacement string }{
		{"wrong-exact", "param#0[1..<3]", "param#0[1..<4]"},
		{"erase", "param#0[1..<3]", "param#0"},
		{"fake-unknown", "param#0[1..<3]", "param#0[?]"},
		{"element-is-not-range", "param#0[1..<3]", "param#0[*]"},
		{"bad-field", "param#0[1..<3]", "param#0.field#99[1..<3]"},
		{"invent-root", "param#0[1..<3]", "param#1[1..<3]"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			forged := bytes.Replace(output, []byte(tc.old), []byte(tc.replacement), 1)
			if bytes.Equal(output, forged) {
				t.Fatal("mutation missed metadata")
			}
			for _, resolve := range []bool{false, true} {
				command := exec.Command(tool, append(sliceVerificationPipeline(resolve), "-")...)
				command.Stdin = bytes.NewReader(forged)
				var stdout, stderr bytes.Buffer
				command.Stdout = &stdout
				command.Stderr = &stderr
				if err := command.Run(); err == nil || stdout.Len() != 0 {
					t.Fatalf("forged range accepted/published: %v\n%s", err, stderr.String())
				}
			}
		})
	}
}
