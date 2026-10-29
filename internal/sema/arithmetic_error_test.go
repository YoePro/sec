package sema

import (
	"reflect"
	"strings"
	"testing"
)

// ArithmeticError is one closed enum with exactly five variants (MD-067):
// Overflow is not a supervariant of the decimal variants, exhaustive
// handlers name all five or use Err(_), and the variants are ordinary enum
// constants.
//
// Rules:
//   - rules/library/core-library.md — § 20.1
//   - rules/errors/errorhandling.md — § 3.4
//   - rules/corrections/md066-md067-md068-decimal-correction-20261028.md — §§ 7(1), 7(6), 7(9), 14 DEC-20
func TestArithmeticErrorHasFiveVariants(t *testing.T) {
	want := []string{"Overflow", "DivisionByZero", "InvalidShift", "ScaleOverflow", "PrecisionExhausted"}
	if got := builtinType("ArithmeticError").EnumValues; !reflect.DeepEqual(got, want) {
		t.Fatalf("ArithmeticError variants = %v, want %v", got, want)
	}
	for name, test := range map[string]struct {
		body string
		want string
	}{
		"all five":  {"fn F(a: int, b: int) int {\n    return try a / b {\n        Err(ArithmeticError.Overflow) => 1\n        Err(ArithmeticError.DivisionByZero) => 2\n        Err(ArithmeticError.InvalidShift) => 3\n        Err(ArithmeticError.ScaleOverflow) => 4\n        Err(ArithmeticError.PrecisionExhausted) => 5\n    }\n}\n", ""},
		"catch-all": {"fn F(a: int, b: int) int {\n    return try a / b {\n        Err(ArithmeticError.DivisionByZero) => 2\n        Err(_) => 0\n    }\n}\n", ""},
		"constants": {"fn F() ArithmeticError {\n    return ArithmeticError.PrecisionExhausted\n}\n", ""},
		"old three are not exhaustive": {"fn F(a: int, b: int) int {\n    return try a / b {\n        Err(ArithmeticError.Overflow) => 1\n        Err(ArithmeticError.DivisionByZero) => 2\n        Err(ArithmeticError.InvalidShift) => 3\n    }\n}\n",
			"try handlers leave ArithmeticError.ScaleOverflow, ArithmeticError.PrecisionExhausted unhandled"},
		"no DecimalArithmeticError": {"fn F() DecimalArithmeticError {\n    return DecimalArithmeticError.Overflow\n}\n", "DecimalArithmeticError"},
	} {
		t.Run(name, func(t *testing.T) {
			errors := analyzeSourceRaw(t, "module main\n"+test.body)
			if test.want == "" {
				if len(errors) != 0 {
					t.Fatalf("want valid, got %v", errors)
				}
				return
			}
			if len(errors) == 0 || !strings.Contains(errors[0].Message, test.want) {
				t.Fatalf("want %q, got %v", test.want, errors)
			}
		})
	}
}
