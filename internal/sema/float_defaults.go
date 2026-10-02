package sema

import (
	"math"
	"math/big"
	"strconv"
)

// binaryFloatRangeDefault selects the valid value nearest zero for a
// range-constrained explicit-width binary float when zero is invalid. The
// candidate from a positive lower bound is the smallest representable value at
// or above it; the candidate from a negative upper bound is the largest
// representable value at or below it, or strictly below it when the bound is
// exclusive. Equal-distance candidates have no implicit default. Plain float
// has no fixed width (MD-014) and therefore no range-derived default.
//
// Rules:
//   - rules/types/default_values.md — "Floating and decimal ranges"
//   - rules/types/default_values.md — "Ambiguous nearest-to-zero values"
//   - rules/types/types.md — "Binary floating-point types"
func binaryFloatRangeDefault(typ Type) DefaultResolution {
	if typ.FloatBits != 32 && typ.FloatBits != 64 {
		return DefaultResolution{Kind: NoDefault}
	}
	var best *DefaultConstant
	tie := false
	for _, contract := range typ.Contracts {
		rangeContract, ok := contract.(RangeContract)
		if !ok {
			continue
		}
		var candidate DefaultConstant
		var found bool
		if rangeContract.ExactMin != nil && rangeContract.ExactMin.Sign() > 0 {
			candidate, found = binaryFloatAtOrAbove(rangeContract.ExactMin, typ.FloatBits)
		} else if rangeContract.ExactMax != nil && rangeContract.ExactMax.Sign() < 0 {
			candidate, found = binaryFloatAtOrBelow(rangeContract.ExactMax, typ.FloatBits, rangeContract.Exclusive)
		}
		if !found || !defaultConstantSatisfies(typ, candidate) {
			continue
		}
		if best == nil {
			best = &candidate
			continue
		}
		switch new(big.Rat).Abs(candidate.Exact).Cmp(new(big.Rat).Abs(best.Exact)) {
		case -1:
			best, tie = &candidate, false
		case 0:
			tie = tie || candidate.Exact.Cmp(best.Exact) != 0
		}
	}
	if best == nil || tie {
		return DefaultResolution{Kind: NoDefault}
	}
	return DefaultResolution{Kind: RangeDefault, Value: *best}
}

// binaryFloatAtOrAbove returns the smallest finite binary float of the given
// width that is greater than or equal to bound.
func binaryFloatAtOrAbove(bound *big.Rat, bits int) (DefaultConstant, bool) {
	value, exact := ratToBinaryFloat(bound, bits)
	if !exact && big.NewRat(0, 1).SetFloat64(value).Cmp(bound) < 0 {
		value = nextBinaryFloat(value, bits, math.Inf(1))
	}
	return binaryFloatConstant(value, bits)
}

// binaryFloatAtOrBelow returns the largest finite binary float of the given
// width that is at most bound, or strictly below bound when exclusive.
func binaryFloatAtOrBelow(bound *big.Rat, bits int, exclusive bool) (DefaultConstant, bool) {
	value, _ := ratToBinaryFloat(bound, bits)
	current := new(big.Rat).SetFloat64(value)
	if current.Cmp(bound) > 0 || exclusive && current.Cmp(bound) == 0 {
		value = nextBinaryFloat(value, bits, math.Inf(-1))
	}
	return binaryFloatConstant(value, bits)
}

func ratToBinaryFloat(value *big.Rat, bits int) (float64, bool) {
	if bits == 32 {
		converted, exact := value.Float32()
		return float64(converted), exact
	}
	return value.Float64()
}

func nextBinaryFloat(value float64, bits int, direction float64) float64 {
	if bits == 32 {
		return float64(math.Nextafter32(float32(value), float32(direction)))
	}
	return math.Nextafter(value, direction)
}

func binaryFloatConstant(value float64, bits int) (DefaultConstant, bool) {
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return DefaultConstant{}, false
	}
	exact := new(big.Rat).SetFloat64(value)
	if exact == nil {
		return DefaultConstant{}, false
	}
	return DefaultConstant{Kind: FloatType, Lexeme: strconv.FormatFloat(value, 'g', -1, bits), Exact: exact}, true
}
