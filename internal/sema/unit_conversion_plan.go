package sema

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/sema/constant"
)

// UnitConversionProof names why an implicit fixed unit conversion is exact in
// its numeric carrier.
//
// Rules:
//   - rules/types/units.md — "Exact fixed conversions", "No hidden precision loss"
type UnitConversionProof string

const (
	// UnitConversionIdentityCoordinate: factor 1 and offset 0, so the
	// coordinate value is unchanged in any carrier.
	UnitConversionIdentityCoordinate UnitConversionProof = "identity-coordinate"
	// UnitConversionExactDecimal: factor and offset are finite decimals, so
	// the decimal carrier represents every converted value exactly.
	UnitConversionExactDecimal UnitConversionProof = "exact-decimal"
	// UnitConversionIntegerValueRange: the compiler-proven value interval of
	// the converted operand, scaled by the factor, stays integral and inside
	// the integer carrier's admitted range.
	UnitConversionIntegerValueRange UnitConversionProof = "integer-value-range"
	// UnitConversionFloatingValueRange proves every admitted binary lattice value exact.
	UnitConversionFloatingValueRange UnitConversionProof = "floating-value-range"
)

// UnitConversionPlan is the compiler-resolved plan of one implicit fixed unit
// conversion: target coordinate = source coordinate * Factor + Offset in the
// unchanged numeric Carrier. Minimum and Maximum are the proven bounds of the
// converted value when Proof is UnitConversionIntegerValueRange.
//
// Rules:
//   - rules/types/units.md — "Exact fixed conversions", "Explicit versus implicit conversion policy"
//   - rules/compiler/semantic_ir.md — unit facts retain the conversion plan
type UnitConversionPlan struct {
	Source       UnitSemantics
	Target       UnitSemantics
	Carrier      Type
	Factor       *big.Rat
	Offset       *big.Rat
	Proof        UnitConversionProof
	Minimum      *big.Int
	Maximum      *big.Int
	FloatMinimum *big.Rat
	FloatMaximum *big.Rat
}

// UnitConversionPlanOf returns the implicit fixed conversion plan Sema chose
// for expr, when expr is implicitly converted between unit identities.
func (a *Analyzer) UnitConversionPlanOf(expr ast.Expression) (UnitConversionPlan, bool) {
	if a == nil || expr == nil {
		return UnitConversionPlan{}, false
	}
	plan, ok := a.unitConversionPlans[expr]
	if !ok {
		return UnitConversionPlan{}, false
	}
	plan.Factor, plan.Offset = cloneRat(plan.Factor), cloneRat(plan.Offset)
	if plan.Minimum != nil {
		plan.Minimum = new(big.Int).Set(plan.Minimum)
		plan.Maximum = new(big.Int).Set(plan.Maximum)
	}
	plan.FloatMinimum, plan.FloatMaximum = cloneRat(plan.FloatMinimum), cloneRat(plan.FloatMaximum)
	return plan, true
}

// sameNumericCarrier reports that two numeric types share one carrier
// representation, independent of unit annotation. A unit annotation never
// makes a carrier change implicit.
//
// Rules:
//   - rules/types/units.md — "Same named unit": numeric carrier conversion may still be needed
//   - rules/types/units.md — "The unit system does not make an otherwise invalid numeric operation valid"
func sameNumericCarrier(left, right Type) bool {
	if left.Kind != right.Kind || numericCarrierName(left) != numericCarrierName(right) {
		return false
	}
	return left.Kind != FloatType || left.FloatBits == right.FloatBits
}

// numericCarrierName is the compiler-known numeric type a value is carried
// in: the underlying carrier of a named numeric type, otherwise the type's own
// name (`int16` for `int16<m>`, `decimal` for `type Distance decimal<m>`).
func numericCarrierName(typ Type) string {
	if typ.Named && typ.Underlying != "" {
		return typ.Underlying
	}
	return typ.Name
}

// unitConversionCoordinates returns the exact affine relation from source to
// target coordinates: target = source*factor + offset.
func unitConversionCoordinates(source, target UnitSemantics) (*big.Rat, *big.Rat, bool) {
	if source.Transform == LogarithmicUnitTransform || target.Transform == LogarithmicUnitTransform {
		if source.Identity == target.Identity && source.Named == target.Named {
			return big.NewRat(1, 1), big.NewRat(0, 1), true
		}
		return nil, nil, false
	}
	factor := unitScaleRatio(source, target)
	offset := big.NewRat(0, 1)
	if source.Role == UnitPointRolePoint || target.Role == UnitPointRolePoint {
		if !unitOriginCompatible(source, target) {
			return nil, nil, false
		}
		sourceOffset, targetOffset := source.Offset, target.Offset
		if sourceOffset == nil {
			sourceOffset = big.NewRat(0, 1)
		}
		if targetOffset == nil {
			targetOffset = big.NewRat(0, 1)
		}
		targetScale := target.Scale
		if targetScale == nil {
			targetScale = big.NewRat(1, 1)
		}
		offset = new(big.Rat).Quo(new(big.Rat).Sub(sourceOffset, targetOffset), targetScale)
	}
	return factor, offset, true
}

// staticUnitConversionPlan proves an implicit fixed conversion exact for every
// value of the carrier.
func staticUnitConversionPlan(source, target UnitSemantics, carrier Type) (UnitConversionPlan, bool) {
	factor, offset, ok := unitConversionCoordinates(source, target)
	if !ok {
		return UnitConversionPlan{}, false
	}
	plan := UnitConversionPlan{Source: source, Target: target, Carrier: carrier, Factor: factor, Offset: offset}
	if factor.Cmp(big.NewRat(1, 1)) == 0 && offset.Sign() == 0 {
		plan.Proof = UnitConversionIdentityCoordinate
		return plan, true
	}
	if carrier.Kind == DecimalType && finiteDecimalRat(factor) && finiteDecimalRat(offset) {
		plan.Proof = UnitConversionExactDecimal
		return plan, true
	}
	return UnitConversionPlan{}, false
}

// valueAwareUnitConversionPlan proves an implicit fixed conversion exact for
// the values expr can actually hold in a bounded integer carrier: its static
// interval (constant, type range narrowed by range contracts and dominating
// comparisons) scaled by the factor must stay integral and inside the carrier
// range admitted by target. A non-integral factor is proven only for a single
// known value it divides exactly. The unchanged carrier evaluates the
// conversion, so a proven plan cannot overflow, truncate, or round.
//
// Binary floating-point carriers use the selected-width lattice proof and finite
// source bounds rather than the integer interval proof.
//
// Rules:
//   - rules/types/units.md — "No hidden precision loss": overflow and truncation are never hidden
//   - rules/types/units.md — "Exact fixed conversions": representable without hidden loss in the chosen carrier
func (a *Analyzer) valueAwareUnitConversionPlan(source, target UnitSemantics, carrier Type, targetType Type, expr ast.Expression) (UnitConversionPlan, bool) {
	if carrier.Kind == FloatType {
		return a.floatingUnitConversionPlan(source, target, carrier, targetType, expr)
	}
	if expr == nil || (carrier.Kind != IntType && carrier.Kind != UintType) {
		return UnitConversionPlan{}, false
	}
	factor, offset, ok := unitConversionCoordinates(source, target)
	if !ok || !offset.IsInt() || factor.Sign() <= 0 {
		return UnitConversionPlan{}, false
	}
	minimum, maximum, known := a.integerInterval(expr)
	if !known {
		return UnitConversionPlan{}, false
	}
	if !factor.IsInt() {
		if minimum.Cmp(maximum) != 0 {
			return UnitConversionPlan{}, false
		}
		scaled := new(big.Int).Mul(minimum, factor.Num())
		if new(big.Int).Rem(scaled, factor.Denom()).Sign() != 0 {
			return UnitConversionPlan{}, false
		}
	}
	convert := func(value *big.Int) *big.Int {
		scaled := new(big.Int).Mul(value, factor.Num())
		scaled.Quo(scaled, factor.Denom())
		return scaled.Add(scaled, offset.Num())
	}
	low, high := convert(minimum), convert(maximum)
	admittedMinimum, admittedMaximum, bounded := integerTypeInterval(targetType)
	if !bounded || low.Cmp(admittedMinimum) < 0 || high.Cmp(admittedMaximum) > 0 {
		return UnitConversionPlan{}, false
	}
	return UnitConversionPlan{
		Source: source, Target: target, Carrier: carrier, Factor: factor, Offset: offset,
		Proof: UnitConversionIntegerValueRange, Minimum: low, Maximum: high,
	}, true
}

// implicitUnitConversionPlan plans the implicit fixed conversion of expr, of
// type value, into target. It reports false when the types are not two
// compatible unit quantities over one carrier with differing coordinates, or
// when no plan is proven exact.
func (a *Analyzer) implicitUnitConversionPlan(target, value Type, expr ast.Expression) (UnitConversionPlan, bool) {
	if !isNumericType(target) || !isNumericType(value) || !hasUnitSemantics(target) || !hasUnitSemantics(value) ||
		isUntypedNumericExpression(expr) || !sameNumericCarrier(target, value) || !target.Dimension.Equal(value.Dimension) {
		return UnitConversionPlan{}, false
	}
	to, from := effectiveUnitSemantics(target), effectiveUnitSemantics(value)
	if !unitKindCompatible(to, from) || !unitOriginCompatible(to, from) || to.Role != from.Role {
		return UnitConversionPlan{}, false
	}
	if plan, ok := staticUnitConversionPlan(from, to, target); ok {
		return plan, true
	}
	return a.valueAwareUnitConversionPlan(from, to, target, target, expr)
}

// canInitialize is canInitialize extended by value-aware implicit unit
// conversion, recording the chosen conversion plan for expr. Speculative
// checks such as overload matching use canInitializeUnrecorded.
func (a *Analyzer) canInitialize(target, value Type, expr ast.Expression) bool {
	// A character literal reaching a char storage site without an expected
	// type is shaped here (rules/types/types.md — "char").
	if value.Kind == RuneType && target.Kind == CharType {
		if typ, shaped := a.shapeCharacterLiteral(expr, target); shaped {
			return typ.Kind != InvalidType
		}
	}
	if !a.canInitializeUnrecorded(target, value, expr) {
		return false
	}
	a.recordUnitConversionPlan(target, value, expr)
	return true
}

func (a *Analyzer) canInitializeUnrecorded(target, value Type, expr ast.Expression) bool {
	if canInitialize(target, value, expr) {
		return true
	}
	_, ok := a.implicitUnitConversionPlan(target, value, expr)
	return ok
}

func (a *Analyzer) recordUnitConversionPlan(target, value Type, expr ast.Expression) {
	if expr == nil || sameConcreteType(target, value) {
		return
	}
	if plan, ok := a.implicitUnitConversionPlan(target, value, expr); ok && plan.Proof != UnitConversionIdentityCoordinate {
		a.unitConversionPlans[expr] = plan
	}
}

// recordCallArgumentUnitConversions records the conversion plans of the
// arguments of a call after overload resolution chose its function.
func (a *Analyzer) recordCallArgumentUnitConversions(function Function, args []ast.Expression, argTypes []Type) {
	for i, arg := range args {
		parameter, ok := functionParameterForArgument(function, i)
		if !ok || i >= len(argTypes) {
			continue
		}
		a.recordUnitConversionPlan(parameter.Type, argTypes[i], arg)
	}
}

// implicitUnitOperandConversion proves the implicit conversion of a binary
// operand (operand, of semantics from) into the other operand's semantics
// (to) in the shared carrier, records its plan, and reports success.
func (a *Analyzer) implicitUnitOperandConversion(operand ast.Expression, from, to UnitSemantics, carrier Type) bool {
	plan, ok := staticUnitConversionPlan(from, to, carrier)
	if !ok {
		// The converted operand must fit the carrier representation; the
		// operand's own range contracts only narrow its source interval.
		representation := Type{Kind: carrier.Kind, Name: carrier.Name, FloatBits: carrier.FloatBits, MinInteger: carrier.MinInteger, MaxInteger: carrier.MaxInteger}
		plan, ok = a.valueAwareUnitConversionPlan(from, to, carrier, representation, operand)
	}
	if ok && plan.Proof != UnitConversionIdentityCoordinate {
		a.unitConversionPlans[operand] = plan
	}
	return ok
}

// implicitUnitConversionRejection explains why no implicit fixed conversion
// plan was proven for expr, naming the value range that failed when the
// carrier is a bounded integer.
func (a *Analyzer) implicitUnitConversionRejection(target, value Type, expr ast.Expression) string {
	const general = "an implicit conversion between unit identities is accepted only when it is exact in the numeric carrier"
	if sameNumericCarrier(target, value) && target.Kind == FloatType && expr != nil {
		if lower, upper, known := a.floatingInterval(expr, value); known {
			return fmt.Sprintf("`%s` may hold %s..%s; its unit conversion is not proven exact without rounding, subnormal loss or overflow in %s (%d bits)", expr.String(), lower.RatString(), upper.RatString(), numericCarrierName(target), target.FloatBits)
		}
		return fmt.Sprintf("`%s` has no proven finite floating interval; unit conversion must be exact in %s (%d bits)", expr.String(), numericCarrierName(target), target.FloatBits)
	}

	if !sameNumericCarrier(target, value) || (target.Kind != IntType && target.Kind != UintType) || expr == nil {
		return general
	}
	factor, offset, ok := unitConversionCoordinates(effectiveUnitSemantics(value), effectiveUnitSemantics(target))
	if !ok || factor.Sign() <= 0 {
		return general
	}
	minimum, maximum, known := a.integerInterval(expr)
	if !known {
		return general
	}
	if !factor.IsInt() || !offset.IsInt() {
		return fmt.Sprintf("the factor %s is exact in %s only for a single known value it divides, and `%s` is not one",
			factor.RatString(), numericCarrierName(target), expr.String())
	}
	values := "may hold " + minimum.String() + ".." + maximum.String()
	if minimum.Cmp(maximum) == 0 {
		values = "holds " + minimum.String()
	}
	return fmt.Sprintf("`%s` %s, which scaled by %s is not proven to fit %s",
		expr.String(), values, factor.RatString(), typeDisplayName(target))
}

// floatingUnitConversionPlan proves both multiplication and the final affine
// result exact, so a cancelling offset cannot hide intermediate rounding or
// overflow. Source contracts constrain the operand, not the result carrier.
// Rules: rules/types/units.md — Exact fixed conversions; No hidden precision loss.
func (a *Analyzer) floatingUnitConversionPlan(source, target UnitSemantics, carrier, targetType Type, expr ast.Expression) (UnitConversionPlan, bool) {
	if expr == nil {
		return UnitConversionPlan{}, false
	}
	typ, ok := a.expressionTypes[expr]
	if !ok || typ.Kind != FloatType {
		return UnitConversionPlan{}, false
	}
	lower, upper, known := a.floatingInterval(expr, typ)
	if !known {
		return UnitConversionPlan{}, false
	}
	factor, offset, ok := unitConversionCoordinates(source, target)
	if !ok {
		return UnitConversionPlan{}, false
	}
	if !constant.FloatAffineIntervalExact(lower, upper, factor, big.NewRat(0, 1), carrier.FloatBits) || !constant.FloatAffineIntervalExact(lower, upper, factor, offset, carrier.FloatBits) {
		return UnitConversionPlan{}, false
	}
	convertedLower := new(big.Rat).Add(new(big.Rat).Mul(lower, factor), offset)
	convertedUpper := new(big.Rat).Add(new(big.Rat).Mul(upper, factor), offset)
	for _, contract := range targetType.Contracts {
		if r, ok := contract.(RangeContract); ok {
			if r.ExactMin != nil && convertedLower.Cmp(r.ExactMin) < 0 || r.ExactMax != nil && (convertedUpper.Cmp(r.ExactMax) > 0 || r.Exclusive && convertedUpper.Cmp(r.ExactMax) == 0) {
				return UnitConversionPlan{}, false
			}
		} else if convertedLower.Cmp(convertedUpper) != 0 || !defaultConstantSatisfies(targetType, DefaultConstant{Kind: FloatType, FloatBits: carrier.FloatBits, Exact: convertedLower}) {
			return UnitConversionPlan{}, false
		}
	}
	return UnitConversionPlan{Source: source, Target: target, Carrier: carrier, Factor: factor, Offset: offset, Proof: UnitConversionFloatingValueRange, FloatMinimum: lower, FloatMaximum: upper}, true
}
