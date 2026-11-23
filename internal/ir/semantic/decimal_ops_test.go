package semantic

import (
	"strings"
	"testing"
)

// P21 §§ 16–20, MD-066 §§ 3–6, MD-067 § 7: decimal operators carry Sema's
// plan into Semantic IR as checked operations that share the integer failure
// CFG; comparison is a total decimal.compare; constant operands fold to an
// exact const.decimal at the selected scale.
func TestDecimalOperatorsInSemanticIR(t *testing.T) {
	module, err := analyzedModule(t, `module main

fn Arithmetic(a: decimal, b: decimal) decimal {
    let sum := a + b
    let product := sum * b
    let quotient := product / a
    return -(quotient % b)
}

fn Wide(a: decimal128, b: decimal128) bool {
    return a - b < b
}

fn Folded() decimal {
    return 1.25m * 2.0m
}

fn Handled(a: decimal, b: decimal) Result[decimal, ArithmeticError] {
    let value := try a / b
    return Ok(value)
}
`, 15)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	text := Format(module)
	for _, want := range []string{
		"decimal.binary-checked add",
		"{coefficient-bits=64, preferred-scale=max-operand-scale, failures=[scale-overflow, precision-exhausted]}",
		"decimal.binary-checked mul",
		"preferred-scale=sum-of-operand-scales",
		"decimal.binary-checked div",
		"quotient=nearest-even, failures=[division-by-zero, scale-overflow, precision-exhausted, overflow]",
		"remainder=truncating",
		"decimal.neg-checked",
		"preferred-scale=operand-scale",
		"coefficient-bits=128",
		"decimal.compare lt",
		"fail.arithmetic",
		"arithmetic-error.from-reason",
		"coefficient=2500 scale=3",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q in:\n%s", want, text)
		}
	}
}

// A named decimal without a fixed scale shares its carrier's decimal type in
// Semantic IR, so its operators are ordinary checked decimal operations.
func TestNamedDecimalOperators(t *testing.T) {
	module, err := analyzedModule(t, `module main

type Price decimal

fn Total(a: Price, b: Price) Price {
    return a + b
}
`, 15)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(module); err != nil {
		t.Fatal(err)
	}
	if text := Format(module); !strings.Contains(text, "decimal.binary-checked add") {
		t.Fatalf("missing decimal add in:\n%s", text)
	}
}
