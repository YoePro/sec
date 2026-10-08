package constant

import (
	"math/big"
	"testing"
)

// TestNumericDomains distinguishes IEEE intermediate rounding from exact
// decimal arithmetic and refuses unrepresentable exact decimal division.
// Rules: rules/foundations/operators.md — Decimal arithmetic, Floating arithmetic;
// rules/compiler/compile_time_evaluation.md — §2(3).
func TestNumericDomains(t *testing.T) {
	for _, bits := range []int{32, 64} {
		x, ok := Numeric("+", big.NewRat(16777216, 1), big.NewRat(1, 1), bits)
		if !ok {
			t.Fatal(bits)
		}
		y, ok := Numeric("-", x, big.NewRat(16777216, 1), bits)
		want := int64(1)
		if bits == 32 {
			want = 0
		}
		if !ok || y.Cmp(big.NewRat(want, 1)) != 0 {
			t.Fatal(bits, y)
		}
	}
	x, ok := Numeric("%", big.NewRat(-13, 4), big.NewRat(2, 1), 0)
	if !ok || x.Cmp(big.NewRat(-5, 4)) != 0 {
		t.Fatal(x, ok)
	}
	if _, ok := DecimalText(big.NewRat(1, 3), 64); ok {
		t.Fatal("inexact decimal accepted")
	}
	if text, ok := DecimalText(big.NewRat(1, 8), 64); !ok || text != "0.125" {
		t.Fatal(text, ok)
	}
	wide := new(big.Int).Lsh(big.NewInt(1), 63)
	if _, ok := DecimalText(new(big.Rat).SetInt(wide), 64); ok {
		t.Fatal("coefficient overflow accepted")
	}
	if _, ok := DecimalText(new(big.Rat).SetInt(wide), 128); !ok {
		t.Fatal("wide decimal rejected")
	}
	if _, ok := Numeric("/", big.NewRat(1, 1), big.NewRat(0, 1), 0); ok {
		t.Fatal("division by zero accepted")
	}
}

// TestFloatEndpointFailures rejects nonfinite range endpoints and unknown
// widths without dereferencing a missing rational value.
// Rules: rules/types/default_values.md — Floating and decimal ranges.
func TestFloatEndpointFailures(t *testing.T) {
	huge := new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), 2000))
	for _, bits := range []int{0, 16, 32, 64} {
		if _, ok := FloatEndpoint(huge, bits, false, false); ok {
			t.Fatal(bits)
		}
	}
}
