package secmlir

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"sec/internal/testsupport"
)

// P21 §§ 16–20 and dialect v17 §§ 3, 6–7: decimal operators emit
// sec.decimal.binary_checked / sec.decimal.unary with Sema's plan and the
// (candidate, failed, reason) results, comparisons emit sec.decimal.cmp, and
// the dialect verifies the module including the guarded failure CFG.
func TestDecimalOperatorEmission(t *testing.T) {
	tool := testsupport.RequireSecMLIROptPath(t)
	source := `module main

fn Arithmetic(a: decimal, b: decimal) decimal {
    let quotient := (a + b) * b / a
    return -(quotient % b)
}

fn Wide(a: decimal128, b: decimal128) bool {
    return a - b >= b
}

fn Handled(a: decimal, b: decimal) Result[decimal, ArithmeticError] {
    let value := try a / b
    return Ok(value)
}

fn main() int {
    return 0
}
`
	module, err := buildPackage19Source(t, source, "amd64", 64)
	if err != nil {
		t.Fatal(err)
	}
	output, err := EmitWithOptions(module, testPlan(64), EmitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	text := string(output)
	for _, want := range []string{
		`"sec.decimal.binary_checked"(%`,
		`<{coefficient_bits = 64 : i32, failures = ["scale-overflow", "precision-exhausted"], kind = "add", preferred_scale = "max-operand-scale", scale_policy = "nearest-exact"}> : (!sec.decimal, !sec.decimal) -> (!sec.decimal, i1, !sec.arithmetic_failure_reason)`,
		`kind = "mul", preferred_scale = "sum-of-operand-scales"`,
		`failures = ["division-by-zero", "scale-overflow", "precision-exhausted", "overflow"], kind = "div", preferred_scale = "max-operand-scale", quotient = "nearest-even"`,
		`remainder = "truncating"`,
		`"sec.decimal.unary"(%`,
		`kind = "neg", preferred_scale = "operand-scale"`,
		`coefficient_bits = 128 : i32`,
		`"sec.decimal.cmp"(%`,
		`<{predicate = "ge"}> : (!sec.decimal128, !sec.decimal128) -> i1`,
		`"sec.arithmetic_error.from_reason"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
	verify := exec.Command(tool, "-", "--sec-verify-checked-integer-guards", "-o", os.DevNull)
	verify.Stdin = bytes.NewReader(output)
	if result, err := verify.CombinedOutput(); err != nil {
		t.Fatalf("%v\n%s\n%s", err, result, text)
	}
}
