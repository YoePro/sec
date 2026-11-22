package sema

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// maxFixedScaleMagnitude bounds the scales whose grid is validated and whose
// default is materialized with exact arithmetic here. Larger fixed scales are
// valid declarations but their grid check and compact default are not
// implemented yet; they are left unchecked rather than approximated.
const maxFixedScaleMagnitude = 1000

// decimalCoefficientLimits returns Cmax = 2^(W-1)-1 and |Cmin| = 2^(W-1).
// Rules: rules/types/contracts.md — § 16; rules/types/default_values.md — § 65A.
func decimalCoefficientLimits(width int) (*big.Int, *big.Int) {
	negative := new(big.Int).Lsh(big.NewInt(1), uint(width-1))
	return new(big.Int).Sub(negative, big.NewInt(1)), negative
}

// Rules: rules/types/contracts.md — § 16; rules/types/default_values.md — § 65A.
func pow10(exponent int64) *big.Int {
	return new(big.Int).Exp(big.NewInt(10), big.NewInt(exponent), nil)
}

// scaledRat returns value × 10^scale exactly.
// Rules: rules/types/contracts.md — § 16; rules/types/default_values.md — § 65A.
func scaledRat(value *big.Rat, scale int32) *big.Rat {
	result := new(big.Rat).Set(value)
	if scale > 0 {
		result.Mul(result, new(big.Rat).SetInt(pow10(int64(scale))))
	} else if scale < 0 {
		result.Quo(result, new(big.Rat).SetInt(pow10(-int64(scale))))
	}
	return result
}

// Rules: rules/types/contracts.md — § 16; rules/types/default_values.md — § 65A.
func ratCeil(value *big.Rat) *big.Int {
	quotient, remainder := new(big.Int).QuoRem(value.Num(), value.Denom(), new(big.Int))
	if remainder.Sign() > 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient
}

// Rules: rules/types/contracts.md — § 16; rules/types/default_values.md — § 65A.
func ratFloor(value *big.Rat) *big.Int {
	quotient, remainder := new(big.Int).QuoRem(value.Num(), value.Denom(), new(big.Int))
	if remainder.Sign() < 0 {
		quotient.Sub(quotient, big.NewInt(1))
	}
	return quotient
}

// decimalBoundFits is the coefficient-capacity envelope of contracts.md
// § 16(3) for one bound value c × 10^(-s) at scale S, decided without
// materializing powers that cannot matter: a magnitude below one coefficient
// step always fits, one far above the limit never does.
func decimalBoundFits(bound DecimalValue, scale int32, width int) bool {
	if bound.Sign() == 0 {
		return true
	}
	limitMax, limitMin := decimalCoefficientLimits(width)
	limit := limitMax
	if bound.Sign() < 0 {
		limit = limitMin
	}
	magnitude := new(big.Int).Abs(bound.Coefficient)
	digits := int64(len(magnitude.String()))
	shift := int64(scale) - int64(bound.Scale)
	if shift >= 0 {
		if digits+shift > int64(len(limit.String()))+1 {
			return false
		}
		return new(big.Int).Mul(magnitude, pow10(shift)).Cmp(limit) <= 0
	}
	if -shift > digits {
		return true // ceil of a magnitude below one coefficient step is 1
	}
	return ratCeil(new(big.Rat).SetFrac(magnitude, pow10(-shift))).Cmp(limit) <= 0
}

// decimalRangeBounds lists every present scale-bearing bound of the type's
// range conjunction, inherited and local, with its source text.
// Rules: rules/types/contracts.md — § 16; rules/types/default_values.md — § 65A.
func decimalRangeBounds(typ Type) ([]DecimalValue, []string) {
	var values []DecimalValue
	var texts []string
	for _, contract := range typ.Contracts {
		r, ok := contract.(RangeContract)
		if !ok {
			continue
		}
		text := func(lexeme string, value DecimalValue) string {
			if lexeme != "" {
				return lexeme
			}
			return decimalTextAtScale(value.Coefficient, value.Scale)
		}
		if r.MinDecimal != nil {
			values, texts = append(values, *r.MinDecimal), append(texts, text(r.MinLexeme, *r.MinDecimal))
		}
		if r.MaxDecimal != nil {
			values, texts = append(values, *r.MaxDecimal), append(texts, text(r.MaxLexeme, *r.MaxDecimal))
		}
	}
	return values, texts
}

// resolveFixedDecimalScale infers the scale of a named decimal type that has
// range bounds but no explicit or inherited scale, then validates the grid of
// every fixed-scale decimal type: each finite bound must fit the coefficient
// domain at the scale and some grid value must satisfy every range (S1065).
// Rules: rules/types/contracts.md — § 16(2)-(5); rules/types/types.md — § 45A(2);
// rules/corrections/applied/MD-049-decimal-range-scale-inference-capacity-correction-rev1-2-2026-11-21-20261122.md — §§ 3, 4, 7, 10.
func (a *Analyzer) resolveFixedDecimalScale(typ Type, token lexer.Token) Type {
	if typ.Kind != DecimalType {
		return typ
	}
	width := a.compileTimeDecimalWidth(typ)
	bounds, texts := decimalRangeBounds(typ)
	scale, fixed := FixedDecimalScaleOf(typ)
	if !fixed {
		if len(bounds) == 0 {
			return typ // variable-scale (contracts.md § 16(2))
		}
		preferred := bounds[0].Scale
		for _, bound := range bounds[1:] {
			preferred = max(preferred, bound.Scale)
		}
		selected, limit := int64(preferred), ""
		limitMax, _ := decimalCoefficientLimits(width)
		for index, bound := range bounds {
			if bound.Sign() == 0 {
				continue
			}
			candidate := min(int64(preferred), int64(bound.Scale)+int64(len(limitMax.String()))-int64(len(new(big.Int).Abs(bound.Coefficient).String()))+1)
			for candidate >= math.MinInt32 && !decimalBoundFits(bound, int32(candidate), width) {
				candidate--
			}
			if candidate < selected {
				selected, limit = candidate, texts[index]
			}
		}
		if selected < math.MinInt32 {
			a.addErrorAtTokenWithMetadata(token, diagnostics.UnsatisfiableContractSet, "narrow the range or use decimal128",
				"no decimal scale in the signed int32 range lets %s hold its range bounds", typeDisplayName(typ))
			return typ
		}
		scale = int32(selected)
		inferred := ScaleContract{Token: token, Value: scale, Inferred: true, Preferred: preferred, Reduced: scale != preferred, Limit: limit, Width: width, Owner: typ.Name}
		typ.Contracts = append(typ.Contracts, inferred)
		a.reportInferredDecimalScale(typ, inferred)
	}
	for index, bound := range bounds {
		if !decimalBoundFits(bound, scale, width) {
			a.addErrorAtTokenWithMetadata(token, diagnostics.UnsatisfiableContractSet, "use a smaller scale, a narrower range or decimal128",
				"range bound %s of %s does not fit a %d-bit coefficient at scale %d", texts[index], typeDisplayName(typ), width, scale)
			return typ
		}
	}
	if lo, hi, ok := fixedScaleCoefficientInterval(typ, scale, width); ok && lo.Cmp(hi) > 0 {
		a.addErrorAtTokenWithMetadata(token, diagnostics.UnsatisfiableContractSet, "widen the range or use a finer scale",
			"no value of %s at scale %d satisfies every range contract", typeDisplayName(typ), scale)
	}
	return typ
}

// reportInferredDecimalScale emits the A2012 information diagnostic.
// Rules: rules/tooling/diagnostics.md — § 31A.
func (a *Analyzer) reportInferredDecimalScale(typ Type, inferred ScaleContract) {
	carrier := "decimal"
	if inferred.Width == 128 {
		carrier = "decimal128"
	}
	message := fmt.Sprintf("%s infers decimal scale %d from its range bounds", typeDisplayName(typ), inferred.Value)
	if inferred.Reduced {
		message = fmt.Sprintf("%s infers decimal scale %d (preferred scale %d): the bound %s needs the integer capacity of %s; the exact range is unchanged", typeDisplayName(typ), inferred.Value, inferred.Preferred, inferred.Limit, carrier)
	}
	endLine, endColumn := inferred.Token.EndPosition()
	a.appendWarning(Error{
		ID: diagnostics.InferredDecimalScale, Severity: diagnostics.SeverityInformation, Message: message,
		Help: fmt.Sprintf("Consider declaring `scale %d` explicitly before the first range.", inferred.Value),
		File: inferred.Token.File, Line: inferred.Token.Line, Column: inferred.Token.Column, EndLine: endLine, EndColumn: endColumn,
	})
}

// fixedScaleCoefficientInterval returns the coefficients c whose value
// c × 10^(-scale) lies in the carrier domain and every range contract: the
// type's grid as one contiguous integer interval. ok is false for scales
// beyond maxFixedScaleMagnitude, which are not evaluated here.
// Rules: rules/types/default_values.md — § 65A(2); rules/types/contracts.md — § 16(4).
func fixedScaleCoefficientInterval(typ Type, scale int32, width int) (*big.Int, *big.Int, bool) {
	if scale > maxFixedScaleMagnitude || scale < -maxFixedScaleMagnitude {
		return nil, nil, false
	}
	limitMax, limitMin := decimalCoefficientLimits(width)
	lo, hi := new(big.Int).Neg(limitMin), new(big.Int).Set(limitMax)
	for _, contract := range typ.Contracts {
		r, ok := contract.(RangeContract)
		if !ok {
			continue
		}
		if r.ExactMin != nil {
			if lower := ratCeil(scaledRat(r.ExactMin, scale)); lower.Cmp(lo) > 0 {
				lo = lower
			}
		}
		if r.ExactMax != nil {
			scaled := scaledRat(r.ExactMax, scale)
			upper := ratFloor(scaled)
			if r.Exclusive {
				upper = new(big.Int).Sub(ratCeil(scaled), big.NewInt(1))
			}
			if upper.Cmp(hi) < 0 {
				hi = upper
			}
		}
	}
	return lo, hi, true
}

// fixedScaleDecimalDefault selects the implicit default on the fixed grid:
// zero when admitted, otherwise the unique member nearest to zero, carried as
// the pair (c, scale).
// Rules: rules/types/default_values.md — § 65A(2)-(3).
func fixedScaleDecimalDefault(typ Type, scale ScaleContract) (DefaultResolution, bool) {
	width := scale.Width
	if width == 0 {
		width = 64
	}
	lo, hi, ok := fixedScaleCoefficientInterval(typ, scale.Value, width)
	if !ok {
		return DefaultResolution{}, false
	}
	if lo.Cmp(hi) > 0 {
		return DefaultResolution{Kind: NoDefault}, true
	}
	coefficient := big.NewInt(0)
	kind := kindForNamed(typ, PrimitiveDefault)
	switch {
	case lo.Sign() > 0:
		coefficient, kind = lo, RangeDefault
	case hi.Sign() < 0:
		coefficient, kind = hi, RangeDefault
	}
	value := fixedScaleConstant(coefficient, scale.Value)
	if !defaultConstantSatisfies(typ, value) {
		return DefaultResolution{Kind: NoDefault}, true
	}
	return DefaultResolution{Kind: kind, Value: value}, true
}

// fixedScaleConstant builds the decimal constant (c, scale) with its exact
// value and canonical text at exactly that scale.
// Rules: rules/types/contracts.md — § 16; rules/types/default_values.md — § 65A.
func fixedScaleConstant(coefficient *big.Int, scale int32) DefaultConstant {
	decimal := DecimalValue{Coefficient: new(big.Int).Set(coefficient), Scale: scale}
	exact := scaledRat(new(big.Rat).SetInt(coefficient), -scale)
	return DefaultConstant{Kind: DecimalType, Exact: exact, Decimal: &decimal, Lexeme: decimalTextAtScale(coefficient, scale)}
}

// decimalTextAtScale spells c × 10^(-scale) with exactly scale fractional
// digits, or with -scale trailing zeros for a negative scale.
// Rules: rules/types/contracts.md — § 16; rules/types/default_values.md — § 65A.
func decimalTextAtScale(coefficient *big.Int, scale int32) string {
	digits := new(big.Int).Abs(coefficient).String()
	sign := ""
	if coefficient.Sign() < 0 {
		sign = "-"
	}
	if scale <= 0 {
		if coefficient.Sign() == 0 {
			return "0"
		}
		return sign + digits + strings.Repeat("0", int(-scale))
	}
	if len(digits) <= int(scale) {
		digits = strings.Repeat("0", int(scale)-len(digits)+1) + digits
	}
	return sign + digits[:len(digits)-int(scale)] + "." + digits[len(digits)-int(scale):]
}

// rescaleToFixedScale places an explicit default exactly on the type's grid,
// or reports that it cannot be placed without rounding or within the
// coefficient width (S1060).
// Rules: rules/types/default_values.md — § 65A(3).
func rescaleToFixedScale(value DefaultConstant, scale ScaleContract) (DefaultConstant, bool) {
	if value.Exact == nil || scale.Value > maxFixedScaleMagnitude || scale.Value < -maxFixedScaleMagnitude {
		return value, value.Exact == nil
	}
	scaled := scaledRat(value.Exact, scale.Value)
	if !scaled.IsInt() {
		return value, false
	}
	width := scale.Width
	if width == 0 {
		width = 64
	}
	limitMax, limitMin := decimalCoefficientLimits(width)
	coefficient := scaled.Num()
	if coefficient.Cmp(limitMax) > 0 || coefficient.Cmp(new(big.Int).Neg(limitMin)) < 0 {
		return value, false
	}
	rescaled := fixedScaleConstant(coefficient, scale.Value)
	rescaled.Token = value.Token
	return rescaled, true
}

// fixedDecimalScaleContract returns the type's ScaleContract, if any.
// Rules: rules/types/contracts.md — § 16; rules/types/default_values.md — § 65A.
func fixedDecimalScaleContract(typ Type) (ScaleContract, bool) {
	for _, contract := range typ.Contracts {
		if scale, ok := contract.(ScaleContract); ok {
			return scale, true
		}
	}
	return ScaleContract{}, false
}

// rescaleConstant replaces value with its exact placement on the grid.
// Rules: rules/types/contracts.md — § 16; rules/types/default_values.md — § 65A.
func rescaleConstant(value *DefaultConstant, scale ScaleContract) bool {
	rescaled, ok := rescaleToFixedScale(*value, scale)
	if ok {
		*value = rescaled
	}
	return ok
}

// FixedDecimalScaleContractOf exposes the frozen fixed-scale fact of a named
// decimal type to Semantic IR and tooling.
// Rules: rules/compiler/semantic_ir.md — § 128(2).
func FixedDecimalScaleContractOf(typ Type) (ScaleContract, bool) {
	return fixedDecimalScaleContract(typ)
}

// checkDecimalConstantConversion validates a compile-time decimal argument
// converted into a named decimal type with contracts, in the order of
// MD-049 § 15(3): placement on the fixed-scale grid (PrecisionLoss), the
// coefficient width at that scale (OutOfRange), then every contract in
// source order. A proven failure is a compile-time S1068 instead of a
// runtime ConversionError. It reports whether the conversion was rejected.
// Rules: rules/types/types.md — § 45A(3); rules/types/contracts.md — §§ 12(1), 16;
// rules/corrections/applied/MD-049-decimal-range-scale-inference-capacity-correction-rev1-2-2026-11-21-20261122.md — § 15(3).
func (a *Analyzer) checkDecimalConstantConversion(call *ast.CallExpression, target Type, argument ast.Expression) bool {
	if target.Kind != DecimalType || !target.Named || !hasContracts(target) {
		return false
	}
	exact, _, outcome := a.semanticCompileTimeExact(argument)
	if outcome != compileTimeEvaluated || exact == nil {
		return false // a runtime value is checked by the runtime conversion
	}
	token := expressionToken(argument)
	value := DefaultConstant{Kind: DecimalType, Exact: exact, Lexeme: argument.String()}
	if scale, fixed := fixedDecimalScaleContract(target); fixed && scale.Value <= maxFixedScaleMagnitude && scale.Value >= -maxFixedScaleMagnitude {
		scaled := scaledRat(exact, scale.Value)
		if !scaled.IsInt() {
			a.addErrorAtTokenWithMetadata(token, diagnostics.ValueViolatesContract, fmt.Sprintf("use a value with at most %d fractional digits, or round it explicitly", scale.Value),
				"%s cannot be converted to %s without rounding to scale %d (ConversionError.PrecisionLoss)", argument.String(), typeDisplayName(target), scale.Value)
			return true
		}
		width := scale.Width
		if width == 0 {
			width = 64
		}
		limitMax, limitMin := decimalCoefficientLimits(width)
		if coefficient := scaled.Num(); coefficient.Cmp(limitMax) > 0 || coefficient.Cmp(new(big.Int).Neg(limitMin)) < 0 {
			a.addErrorAtTokenWithMetadata(token, diagnostics.ValueViolatesContract, "use a smaller value or decimal128",
				"%s does not fit a %d-bit coefficient of %s at scale %d (ConversionError.OutOfRange)", argument.String(), width, typeDisplayName(target), scale.Value)
			return true
		}
	}
	if violated, ok := firstViolatedContract(target, value); ok {
		a.addContractError(token, violated, diagnostics.ValueViolatesContract, "use a value satisfying every contract of the named type",
			"%s violates %s of %s", argument.String(), describeContract(violated), typeDisplayName(target))
		return true
	}
	// The proven value, placed on the target's grid, is the conversion's
	// compile-time result.
	pair, known := a.semanticCompileTimeDecimal(argument)
	if scale, fixed := fixedDecimalScaleContract(target); fixed {
		if rescaled, ok := rescaleToFixedScale(value, scale); ok && rescaled.Decimal != nil {
			pair, known = *rescaled.Decimal, true
		}
	}
	if known {
		if a.decimalConstantConversions == nil {
			a.decimalConstantConversions = map[*ast.CallExpression]DecimalValue{}
		}
		a.decimalConstantConversions[call] = pair
	}
	return false
}

// ResolvedDecimalConstantConversionOf returns the compile-time result of a
// proven conversion of a decimal constant into a named decimal type, as its
// (coefficient, scale) pair on the target's grid.
// Rules: rules/types/contracts.md — § 12(1); rules/compiler/semantic_ir.md — § 128(2).
func (a *Analyzer) ResolvedDecimalConstantConversionOf(call *ast.CallExpression) (DecimalValue, bool) {
	pair, ok := a.decimalConstantConversions[call]
	if !ok || pair.Coefficient == nil {
		return DecimalValue{}, false
	}
	return DecimalValue{Coefficient: new(big.Int).Set(pair.Coefficient), Scale: pair.Scale}, true
}
