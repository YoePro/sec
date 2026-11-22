package constant

import (
	"math/big"
	"testing"
)

// MD-066 §§ 3–5 and MD-067 § 7 decimal evaluation, including the
// correction's representative division table (§ 5(6)).
func TestEvaluateDecimal(t *testing.T) {
	type operand struct {
		coefficient string
		scale       int64
	}
	cases := []struct {
		name      string
		op        string
		a, b      operand
		want      string
		wantScale int64
		failure   DecimalFailure
		bits      int
	}{
		{"add keeps max scale", "+", operand{"12", 1}, operand{"100", 2}, "220", 2, "", 64},
		{"sub keeps max scale", "-", operand{"120", 2}, operand{"12", 1}, "0", 2, "", 64},
		{"mul sums scales", "*", operand{"15", 1}, operand{"25", 1}, "375", 2, "", 64},
		{"1/3", "/", operand{"1", 0}, operand{"3", 0}, "3333333333333333333", 19, "", 64},
		{"2/3 nearest even", "/", operand{"2", 0}, operand{"3", 0}, "6666666666666666667", 19, "", 64},
		{"1e9/3", "/", operand{"1000000000", 0}, operand{"3", 0}, "3333333333333333333", 10, "", 64},
		{"1/8 exact", "/", operand{"1", 0}, operand{"8", 0}, "125", 3, "", 64},
		{"6/3 exact keeps preferred", "/", operand{"60", 1}, operand{"3", 0}, "20", 1, "", 64},
		{"remainder truncates", "%", operand{"-75", 1}, operand{"2", 0}, "-15", 1, "", 64},
		{"division by zero", "/", operand{"1", 0}, operand{"0", 2}, "", 0, DecimalDivisionByZero, 64},
		{"remainder by zero", "%", operand{"1", 0}, operand{"0", 0}, "", 0, DecimalDivisionByZero, 64},
		{"preferred coefficient overflow uses coarser exact scale", "*", operand{"9000000000", 0}, operand{"1000000000000", 0}, "9000000000000000000", -3, "", 64},
		{"exact digits exhausted", "+", operand{"9223372036854775807", 0}, operand{"1", 0}, "", 0, DecimalPrecisionExhausted, 64},
		{"decimal128 holds the sum", "+", operand{"9223372036854775807", 0}, operand{"1", 0}, "9223372036854775808", 0, "", 128},
		{"whole part too large", "/", operand{"92233720368547758070", -1}, operand{"3", 0}, "", 0, DecimalPrecisionExhausted, 64},
		// MD-049 § 12(8), operators.md § 27.1(5a): an exact zero product keeps
		// the greater operand scale (T31-T36), symmetrically; a nonzero product
		// keeps the scale sum.
		{"T32 zero product keeps max scale", "*", operand{"0", 2}, operand{"0", 3}, "0", 3, "", 64},
		{"T33 zero product is symmetric", "*", operand{"0", 3}, operand{"0", 2}, "0", 3, "", 64},
		{"T34 equal zero scales do not add", "*", operand{"0", 3}, operand{"0", 3}, "0", 3, "", 64},
		{"T35 zero times nonzero", "*", operand{"0", 2}, operand{"1234", 3}, "0", 3, "", 64},
		{"T31 nonzero product keeps scale sum", "*", operand{"120", 2}, operand{"20", 1}, "2400", 3, "", 64},
		{"T36 zero sum keeps max scale", "+", operand{"0", 2}, operand{"0", 3}, "0", 3, "", 64},
		{"zero product with negative scale", "*", operand{"0", -2}, operand{"5", 1}, "0", 1, "", 64},
		{"decimal128 zero product", "*", operand{"0", 4}, operand{"0", 6}, "0", 6, "", 128},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			a, _ := new(big.Int).SetString(test.a.coefficient, 10)
			b, _ := new(big.Int).SetString(test.b.coefficient, 10)
			coefficient, scale, failure, ok := EvaluateDecimal(test.op, a, test.a.scale, b, test.b.scale, test.bits)
			if !ok {
				t.Fatalf("not evaluated")
			}
			if failure != test.failure {
				t.Fatalf("failure = %q, want %q", failure, test.failure)
			}
			if failure != "" {
				return
			}
			if coefficient.String() != test.want || scale != test.wantScale {
				t.Fatalf("got %s scale %d, want %s scale %d", coefficient, scale, test.want, test.wantScale)
			}
		})
	}
}

// MD-049 § 12(9), test T50: multiplying a zero repeatedly by 0.000 never grows
// its scale past the greatest input scale.
func TestRepeatedZeroProductScaleStaysBounded(t *testing.T) {
	coefficient, scale := big.NewInt(0), int64(2)
	for iteration := 0; iteration < 10; iteration++ {
		var failure DecimalFailure
		var ok bool
		coefficient, scale, failure, ok = EvaluateDecimal("*", coefficient, scale, big.NewInt(0), 3, 64)
		if !ok || failure != "" || coefficient.Sign() != 0 || scale != 3 {
			t.Fatalf("iteration %d: %s scale %d (%q, %v)", iteration, coefficient, scale, failure, ok)
		}
	}
}
