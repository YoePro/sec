package constant

import (
	"math"
	"math/big"
	"testing"
)

// TestFloatConversionLattices checks exact and inexact transformations around
// finite extrema, subnormal spacing and mantissa boundaries at both widths.
// Rules: rules/types/units.md — No hidden precision loss.
func TestFloatConversionLattices(t *testing.T) {
	for _, bits := range []int{32, 64} {
		zero := big.NewRat(0, 1)
		cases := []struct {
			lo, hi, factor, offset *big.Rat
			want                   bool
		}{
			{big.NewRat(-1, 1), big.NewRat(1, 1), big.NewRat(2, 1), zero, true},
			{big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(1, 2), zero, true},
			{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(1, 2), zero, false},
			{big.NewRat(10, 1), big.NewRat(10, 1), big.NewRat(1, 10), zero, true},
			{big.NewRat(1, 1), big.NewRat(1, 1), big.NewRat(1, 10), zero, false},
			{big.NewRat(1, 1), big.NewRat(2, 1), big.NewRat(3, 1), zero, false},
			{zero, zero, big.NewRat(1, 1), big.NewRat(1, 2), true},
		}
		for _, c := range cases {
			if got := FloatAffineIntervalExact(c.lo, c.hi, c.factor, c.offset, bits); got != c.want {
				t.Fatalf("%d %+v: %v", bits, c, got)
			}
		}
		maximum := math.MaxFloat64
		tiniest := math.SmallestNonzeroFloat64
		if bits == 32 {
			maximum = math.MaxFloat32
			tiniest = math.SmallestNonzeroFloat32
		}
		max, _ := FloatExact(maximum, bits)
		tiny, _ := FloatExact(tiniest, bits)
		if FloatAffineIntervalExact(max, max, big.NewRat(2, 1), zero, bits) {
			t.Fatal("finite overflow")
		}
		if FloatAffineIntervalExact(tiny, tiny, big.NewRat(1, 2), zero, bits) {
			t.Fatal("underflow")
		}
	}
}

// TestFloatConversionProofHasNoFalsePositives checks small adjacent-value
// windows with an independent exact-rational oracle at difficult IEEE boundaries.
// Rules: rules/types/units.md — No hidden precision loss.
func TestFloatConversionProofHasNoFalsePositives(t *testing.T) {
	for _, bits := range []int{32, 64} {
		starts := []float64{0, 1, -1, math.SmallestNonzeroFloat64, math.MaxFloat64 / 2}
		if bits == 32 {
			starts = []float64{0, 1, -1, math.SmallestNonzeroFloat32, float64(float32(math.MaxFloat32 / 2))}
		}
		for _, start := range starts {
			values := []*big.Rat{}
			value := start
			for i := 0; i < 9; i++ {
				exact, ok := FloatExact(value, bits)
				if !ok {
					break
				}
				values = append(values, exact)
				if bits == 32 {
					value = float64(math.Nextafter32(float32(value), float32(math.Inf(1))))
				} else {
					value = math.Nextafter(value, math.Inf(1))
				}
			}
			for _, factor := range []*big.Rat{big.NewRat(2, 1), big.NewRat(1, 2), big.NewRat(3, 1), big.NewRat(1, 10)} {
				for _, offset := range []*big.Rat{big.NewRat(0, 1), big.NewRat(1, 1), big.NewRat(-1, 1)} {
					if !FloatAffineIntervalExact(values[0], values[len(values)-1], factor, offset, bits) {
						continue
					}
					for _, x := range values {
						image := new(big.Rat).Add(new(big.Rat).Mul(x, factor), offset)
						var f float64
						if bits == 32 {
							v, _ := image.Float32()
							f = float64(v)
						} else {
							f, _ = image.Float64()
						}
						roundtrip, ok := FloatExact(f, bits)
						if !ok || roundtrip.Cmp(image) != 0 {
							t.Fatalf("false proof at %d bits: %s -> %s", bits, x, image)
						}
					}
				}
			}
		}
	}
}
