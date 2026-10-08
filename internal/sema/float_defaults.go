package sema

import (
	"math/big"
	"sec/internal/sema/constant"
	"strconv"
)

// binaryFloatRangeDefault selects the valid value nearest zero for a
// range-constrained binary float when zero is invalid. The candidate from a
// positive lower bound is the smallest representable value at or above it; the
// candidate from a negative upper bound is the largest representable value at
// or below it, or strictly below it when the bound is exclusive. Equal-distance
// candidates have no implicit default. Plain float uses its platform-selected
// width (MD-014), so it resolves like float32 or float64.
//
// Rules:
//   - rules/corrections/applied/missing-decisions-md010-md014-correction-20261003.md — §§ 6.14–6.17
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
	value, ok := constant.FloatEndpoint(bound, bits, false, false)
	if !ok {
		return DefaultConstant{}, false
	}
	return binaryFloatConstant(value, bits)
}

// binaryFloatAtOrBelow returns the largest finite binary float of the given
// width that is at most bound, or strictly below bound when exclusive.
func binaryFloatAtOrBelow(bound *big.Rat, bits int, exclusive bool) (DefaultConstant, bool) {
	value, ok := constant.FloatEndpoint(bound, bits, true, exclusive)
	if !ok {
		return DefaultConstant{}, false
	}
	return binaryFloatConstant(value, bits)
}

// binaryFloatConstant adapts finite binary scalar values for default resolution.
// Rules: rules/types/default_values.md — Floating and decimal ranges.
func binaryFloatConstant(value float64, bits int) (DefaultConstant, bool) {
	exact, ok := constant.FloatExact(value, bits)
	if !ok {
		return DefaultConstant{}, false
	}
	return DefaultConstant{Kind: FloatType, FloatBits: bits, Lexeme: strconv.FormatFloat(value, 'g', -1, bits), Exact: exact}, true
}
