package constant

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// maxDecimalScaleShift bounds how many powers of ten a literal may be
// rescaled by to reach the int32 scale domain; a larger shift could never
// yield a representable coefficient, so it is rejected without materializing
// 10^N.
const maxDecimalScaleShift = 200

// DecimalLiteral reads the exact value of a decimal-form literal spelling
// (digits, an optional fraction and an optional exponent; no suffix, no
// digit separators) as its coefficient and the scale its spelling implies:
// the number of fractional digits minus the exponent. `1.20` has scale 2,
// `1.25e-3` scale 5 and `1e3` scale -3. It never passes through binary
// floating point.
// Rules: rules/types/types.md — "Decimal scale";
// rules/corrections/md066-md067-md068-decimal-correction-20261028.md — § 3(7).
func DecimalLiteral(spelling string) (coefficient *big.Int, scale int64, ok bool) {
	negative := strings.HasPrefix(spelling, "-")
	spelling = strings.TrimPrefix(spelling, "-")
	exponent := int64(0)
	if index := strings.IndexAny(spelling, "eE"); index >= 0 {
		parsed, err := strconv.ParseInt(spelling[index+1:], 10, 32)
		if err != nil {
			return nil, 0, false
		}
		exponent, spelling = parsed, spelling[:index]
	}
	whole, fraction, _ := strings.Cut(spelling, ".")
	digits := whole + fraction
	if digits == "" || strings.ContainsAny(digits, ".+-") {
		return nil, 0, false
	}
	coefficient, ok = new(big.Int).SetString(digits, 10)
	if !ok {
		return nil, 0, false
	}
	if negative {
		coefficient.Neg(coefficient)
	}
	return coefficient, int64(len(fraction)) - exponent, true
}

// FitDecimal selects the legal representation of the exact value
// coefficient × 10^(-scale) whose scale is closest to the preferred scale:
// an int32 scale and, when bits is 64 or 128, a coefficient in that signed
// width (bits 0 leaves the coefficient width open). A zero keeps the
// preferred scale clamped to int32. It reports false when no exact legal
// representation exists.
// Rules: rules/types/types.md — "Decimal scale";
// rules/foundations/operators.md — "Decimal result scale" (MD-066 §§ 3(3)–3(7)).
func FitDecimal(coefficient *big.Int, scale int64, bits int) (*big.Int, int32, bool) {
	if coefficient == nil {
		return nil, 0, false
	}
	if coefficient.Sign() == 0 {
		return new(big.Int), int32(min(max(scale, math.MinInt32), math.MaxInt32)), true
	}
	fits := func(c *big.Int) bool {
		if bits == 0 {
			return true
		}
		limit := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
		return c.Cmp(new(big.Int).Neg(limit)) >= 0 && c.Cmp(limit) < 0
	}
	ten := big.NewInt(10)
	if scale < math.MinInt32 {
		// Only a finer scale is legal: multiply by the missing powers of ten.
		shift := math.MinInt32 - scale
		if shift > maxDecimalScaleShift {
			return nil, 0, false
		}
		c := new(big.Int).Mul(coefficient, new(big.Int).Exp(ten, big.NewInt(shift), nil))
		return c, math.MinInt32, fits(c)
	}
	// Coarser exact scales strip trailing zeros; the closest legal one is the
	// largest scale not above both the preferred scale and int32.Max.
	c := new(big.Int).Set(coefficient)
	s := scale
	for s > math.MaxInt32 || !fits(c) {
		q, r := new(big.Int).QuoRem(c, ten, new(big.Int))
		if r.Sign() != 0 || s-1 < math.MinInt32 {
			return nil, 0, false
		}
		c, s = q, s-1
	}
	return c, int32(s), true
}
