package sema

import (
	"strings"
	"testing"
)

// A decimal divisor the compiler proves zero is a compile-time error for /
// and %, with or without try; a divisor that is not provably zero keeps the
// runtime check and is accepted.
//
// Rules:
//   - rules/foundations/operators.md — "Decimal division and remainder by zero"
//   - rules/errors/runtime_checks.md — "Division and remainder"
//   - rules/corrections/md066-md067-md068-decimal-correction-20261028.md — §§ 6(1), 6(4)–6(5), 14 DEC-10
func TestDecimalProvablyZeroDivisor(t *testing.T) {
	for name, test := range map[string]struct {
		body string
		want string
	}{
		"literal":           {"fn F(x: decimal) decimal {\n    return x / 0.0m\n}\n", "decimal division by zero"},
		"integer literal":   {"fn F(x: decimal) decimal {\n    return x / 0\n}\n", "decimal division by zero"},
		"negative zero":     {"fn F(x: decimal) decimal {\n    return x % -0.00m\n}\n", "decimal remainder by zero"},
		"under try":         {"fn F(x: decimal) Result[decimal, ArithmeticError] {\n    let q := try x / 0m\n    return Ok(q)\n}\n", "decimal division by zero"},
		"immutable binding": {"fn F(x: decimal) decimal {\n    let zero := 1.5m - 1.50m\n    return x / zero\n}\n", "decimal division by zero"},
		"decimal128":        {"fn F(x: decimal128) decimal128 {\n    return x / 0m\n}\n", "decimal division by zero"},
		"nonzero literal":   {"fn F(x: decimal) decimal {\n    return x / 0.5m\n}\n", ""},
		"parameter":         {"fn F(x: decimal, y: decimal) decimal {\n    return x / y\n}\n", ""},
		"mutable binding":   {"fn F(x: decimal) decimal {\n    let mut d := 0m\n    d = 2m\n    return x / d\n}\n", ""},
		"zero dividend":     {"fn F(x: decimal) decimal {\n    return 0m / x\n}\n", ""},
	} {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n"+test.body)
			if test.want == "" {
				if len(errors) != 0 {
					t.Fatalf("want valid, got %v", errors)
				}
				return
			}
			if len(errors) == 0 || !strings.Contains(errors[0].Message, test.want) || errors[0].ID != "" && !strings.HasPrefix(errors[0].ID, "S102") {
				t.Fatalf("want %q (S1024/S1025), got %+v", test.want, errors)
			}
		})
	}
}

// Compound decimal division and remainder by a provably zero divisor are
// rejected once, like the binary operators.
// Rules: rules/foundations/operators.md — "Decimal division and remainder by zero"; MD-066 § 4(4).
func TestDecimalProvablyZeroDivisorCompound(t *testing.T) {
	for _, operator := range []string{"/=", "%="} {
		errors := analyzeSourceRaw(t, "module main\nfn F(x: decimal) decimal {\n    let mut y := x\n    y "+operator+" 0.0m\n    return y\n}\n")
		if len(errors) != 1 || !strings.Contains(errors[0].Message, "by zero") {
			t.Errorf("%s: want one zero-divisor error, got %v", operator, errors)
		}
	}
	if errors := analyzeSourceRaw(t, "module main\nfn F(x: decimal) decimal {\n    return x / 0m\n}\n"); len(errors) != 1 {
		t.Errorf("binary division reported %d errors, want 1: %v", len(errors), errors)
	}
}
