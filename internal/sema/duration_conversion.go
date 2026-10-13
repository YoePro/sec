package sema

import (
	"fmt"
	"math/big"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// DurationConversionProof names why a time-quantity-to-duration conversion
// is valid.
type DurationConversionProof string

const (
	// DurationConversionConstant: the exact value is known at compile time
	// and its (rounded, when a mode is given) nanosecond count fits int64.
	DurationConversionConstant DurationConversionProof = "constant"
	// DurationConversionBoundedDomain: every value of the statically known
	// integer domain converts to an integral in-range nanosecond count.
	DurationConversionBoundedDomain DurationConversionProof = "bounded-domain"
	// DurationConversionChecked: an explicit conversion whose validity cannot
	// be proved; it is checked at run time under try and fails with
	// ConversionError.
	DurationConversionChecked DurationConversionProof = "checked"
)

// DurationConversionPlan is the compiler-resolved conversion of one time
// quantity to duration: nanoseconds = value * NanosecondsPerUnit, rounded once
// by Rounding when one is given, then checked against the signed 64-bit
// nanosecond range.
//
// Rules:
//   - rules/types/temporal.md — "Conversion of time quantities to duration"
//   - rules/types/units.md — "Quantity literals"
type DurationConversionPlan struct {
	Source             Type
	Unit               UnitSemantics
	NanosecondsPerUnit *big.Rat
	// Rounding is the RoundingMode case name, or "" for an exact conversion.
	Rounding string
	Proof    DurationConversionProof
	Explicit bool
	// Nanoseconds is the result of a constant conversion.
	Nanoseconds *big.Int
	// Minimum and Maximum bound a bounded-domain conversion's result.
	Minimum, Maximum *big.Int
	// Failure is the ConversionError variant a checked conversion reports
	// first.
	Failure ConversionFailureKind
}

// DurationConversionPlanOf returns the duration conversion Sema resolved for
// an explicit duration(...) call or a contextually converted expression.
func (a *Analyzer) DurationConversionPlanOf(expr ast.Expression) (DurationConversionPlan, bool) {
	if a == nil || expr == nil {
		return DurationConversionPlan{}, false
	}
	plan, ok := a.durationConversions[expr]
	return plan, ok
}

var (
	durationMinimum      = big.NewInt(-1 << 63)
	durationMaximum      = new(big.Int).SetUint64(1<<63 - 1)
	nanosecondsPerSecond = big.NewRat(1_000_000_000, 1)
)

func (a *Analyzer) isDurationType(typ Type) bool {
	core, ok := a.types["duration"]
	return ok && typ.Kind == StructType && typ.Name == "duration" && sameConcreteType(typ, core)
}

// timeQuantityUnit returns the unit facts of a numeric quantity that is a
// linear time difference, the only kind of quantity convertible to duration.
// Spelling never matters: only the resolved dimension, Kind, transform and
// exact scale do.
func timeQuantityUnit(typ Type) (UnitSemantics, string, bool) {
	if !isNumericType(typ) || !hasUnitSemantics(typ) {
		return UnitSemantics{}, "", false
	}
	if len(typ.Dimension.Base) != 1 || typ.Dimension.Base["time"] != 1 {
		return UnitSemantics{}, fmt.Sprintf("%s does not have the dimension of time", typeDisplayName(typ)), false
	}
	unit := effectiveUnitSemantics(typ)
	if unit.Kind != "" && unit.Kind != "time" {
		return UnitSemantics{}, fmt.Sprintf("%s has Kind %s, not time", typeDisplayName(typ), unit.Kind), false
	}
	if unit.Role == UnitPointRolePoint {
		return UnitSemantics{}, fmt.Sprintf("%s is a time point, not a time difference", typeDisplayName(typ)), false
	}
	if unit.Transform != "" && unit.Transform != LinearUnitTransform || unit.Offset != nil && unit.Offset.Sign() != 0 {
		return UnitSemantics{}, fmt.Sprintf("%s has a non-linear transform", typeDisplayName(typ)), false
	}
	if unit.Scale == nil || unit.Scale.Sign() <= 0 {
		return UnitSemantics{}, fmt.Sprintf("%s has no resolved compile-time scale", typeDisplayName(typ)), false
	}
	return unit, "", true
}

// exactQuantityValue evaluates the exact value of a quantity expression in
// its own unit coordinates: quantity and numeric literals, signs and
// immutable bindings with a recorded exact value.
func (a *Analyzer) exactQuantityValue(expr ast.Expression) (*big.Rat, bool) {
	switch node := expr.(type) {
	case *ast.QuantityLiteral:
		value, _, ok := exactNumericConstant(node.Value)
		return value, ok
	case *ast.IntegerLiteral, *ast.FloatLiteral:
		value, _, ok := exactNumericConstant(node)
		return value, ok
	case *ast.PrefixExpression:
		if node.Operator != "-" && node.Operator != "+" {
			return nil, false
		}
		value, ok := a.exactQuantityValue(node.Right)
		if !ok {
			return nil, false
		}
		if node.Operator == "-" {
			value = new(big.Rat).Neg(value)
		}
		return value, true
	case *ast.Identifier:
		symbol, ok := a.symbols[node.Value]
		if !ok || symbol.Mutable || symbol.Volatile || symbol.QuantityConstant == nil {
			return nil, false
		}
		return new(big.Rat).Set(symbol.QuantityConstant), true
	}
	return nil, false
}

// recordQuantityBindingConstant snapshots the exact value of an immutable
// unit-bearing binding so a later duration conversion of it can be proved.
func (a *Analyzer) recordQuantityBindingConstant(name string, expr ast.Expression) {
	symbol, ok := a.symbols[name]
	if !ok || symbol.Mutable || symbol.Volatile || !hasUnitSemantics(symbol.Type) {
		return
	}
	value, ok := a.exactQuantityValue(expr)
	if !ok {
		return
	}
	if plan, planned := a.UnitConversionPlanOf(expr); planned {
		value = new(big.Rat).Add(new(big.Rat).Mul(value, plan.Factor), plan.Offset)
	}
	symbol.QuantityConstant = value
	a.symbols[name] = symbol
}

// roundRat applies one RoundingMode to an exact rational value.
func roundRat(value *big.Rat, mode string) (*big.Int, bool) {
	if value.IsInt() {
		return new(big.Int).Set(value.Num()), true
	}
	floor := new(big.Int).Div(value.Num(), value.Denom()) // Euclidean: floor for a positive denominator
	ceil := new(big.Int).Add(floor, big.NewInt(1))
	half := new(big.Rat).Sub(value, new(big.Rat).SetInt(floor)).Cmp(big.NewRat(1, 2))
	towardZero, awayFromZero := floor, ceil
	if value.Sign() < 0 {
		towardZero, awayFromZero = ceil, floor
	}
	switch mode {
	case "Floor":
		return floor, true
	case "Ceil":
		return ceil, true
	case "TowardZero":
		return towardZero, true
	case "AwayFromZero":
		return awayFromZero, true
	case "NearestAwayFromZero":
		switch {
		case half < 0:
			return floor, true
		case half > 0:
			return ceil, true
		}
		return awayFromZero, true
	case "NearestEven":
		switch {
		case half < 0:
			return floor, true
		case half > 0:
			return ceil, true
		}
		if floor.Bit(0) == 0 {
			return floor, true
		}
		return ceil, true
	}
	return nil, false
}

// constantDurationNanoseconds converts an exact value; reason explains a
// failure (inexact without a rounding mode, or outside int64).
func constantDurationNanoseconds(value, perUnit *big.Rat, rounding string) (*big.Int, ConversionFailureKind, string) {
	exact := new(big.Rat).Mul(value, perUnit)
	var nanoseconds *big.Int
	if rounding == "" {
		if !exact.IsInt() {
			return nil, ConversionFailurePrecisionLoss, fmt.Sprintf("it is %s nanoseconds, not a whole number; pass an explicit RoundingMode", exact.RatString())
		}
		nanoseconds = new(big.Int).Set(exact.Num())
	} else {
		nanoseconds, _ = roundRat(exact, rounding)
	}
	if nanoseconds.Cmp(durationMinimum) < 0 || nanoseconds.Cmp(durationMaximum) > 0 {
		return nil, ConversionFailureOutOfRange, fmt.Sprintf("%s nanoseconds is outside the signed 64-bit duration range", nanoseconds.String())
	}
	return nanoseconds, "", ""
}

// planDurationConversion proves or classifies the conversion of expr, of
// type source, to duration. explicit is true for duration(...); a contextual
// conversion is valid only when proved.
func (a *Analyzer) planDurationConversion(expr ast.Expression, source Type, rounding string, explicit bool) (DurationConversionPlan, string, bool) {
	unit, reason, ok := timeQuantityUnit(source)
	if !ok {
		if reason == "" {
			reason = fmt.Sprintf("%s is not a time quantity", typeDisplayName(source))
		}
		return DurationConversionPlan{}, reason, false
	}
	plan := DurationConversionPlan{Source: source, Unit: unit, NanosecondsPerUnit: new(big.Rat).Mul(unit.Scale, nanosecondsPerSecond), Rounding: rounding, Explicit: explicit}
	if value, known := a.exactQuantityValue(expr); known {
		nanoseconds, _, failure := constantDurationNanoseconds(value, plan.NanosecondsPerUnit, rounding)
		if failure != "" {
			return DurationConversionPlan{}, failure, false
		}
		plan.Proof, plan.Nanoseconds = DurationConversionConstant, nanoseconds
		return plan, "", true
	}
	if isIntegerType(source) && (plan.NanosecondsPerUnit.IsInt() || rounding != "") {
		if minimum, maximum, known := a.scalarValueInterval(expr, source); known {
			low, _, _ := constantDurationNanoseconds(new(big.Rat).SetInt(minimum), plan.NanosecondsPerUnit, rounding)
			high, _, _ := constantDurationNanoseconds(new(big.Rat).SetInt(maximum), plan.NanosecondsPerUnit, rounding)
			if low != nil && high != nil {
				plan.Proof, plan.Minimum, plan.Maximum = DurationConversionBoundedDomain, low, high
				return plan, "", true
			}
		}
	}
	if !explicit {
		return DurationConversionPlan{}, fmt.Sprintf("the compiler cannot prove that every value of %s converts exactly to an in-range duration; write try duration(...) to check it", typeDisplayName(source)), false
	}
	plan.Proof = DurationConversionChecked
	plan.Failure = ConversionFailureOutOfRange
	if rounding == "" && (!isIntegerType(source) || !plan.NanosecondsPerUnit.IsInt()) {
		plan.Failure = ConversionFailurePrecisionLoss
	}
	return plan, "", true
}

// inferDurationConversion handles duration(q) and duration(q, mode).
func (a *Analyzer) inferDurationConversion(expr *ast.CallExpression, target Type) (Type, expressionValue) {
	display := expressionValue{Display: expr.String()}
	if len(expr.Arguments) != 1 && len(expr.Arguments) != 2 {
		a.addErrorAtToken(expr.Token, "conversion to duration expects a time quantity and an optional RoundingMode, got %d arguments", len(expr.Arguments))
		return Type{Kind: InvalidType}, display
	}
	source, _ := a.inferExpression(expr.Arguments[0])
	if source.Kind == InvalidType {
		return Type{Kind: InvalidType}, display
	}
	if a.isDurationType(source) && len(expr.Arguments) == 1 {
		return target, display
	}
	rounding := ""
	if len(expr.Arguments) == 2 {
		mode, ok := a.roundingModeArgument(expr.Arguments[1])
		if !ok {
			return Type{Kind: InvalidType}, display
		}
		rounding = mode
	}
	plan, reason, ok := a.planDurationConversion(expr.Arguments[0], source, rounding, true)
	if !ok {
		a.addErrorAtTokenWithMetadata(expr.Token, diagnostics.DurationConversionInvalid, durationConversionHelp,
			"cannot convert %s to duration: %s", expr.Arguments[0].String(), reason)
		return Type{Kind: InvalidType}, display
	}
	a.durationConversions[expr] = plan
	if plan.Proof == DurationConversionChecked {
		a.checkedConversions[expr] = checkedConversion{source: source, target: target, failure: plan.Failure}
		a.fallibleConversionRelations[expr] = plan.Failure
	} else {
		delete(a.checkedConversions, expr)
		delete(a.fallibleConversionRelations, expr)
	}
	return target, display
}

// roundingModeArgument accepts only a constant RoundingMode case.
func (a *Analyzer) roundingModeArgument(arg ast.Expression) (string, bool) {
	modeType, _ := a.inferExpression(arg)
	if modeType.Kind == InvalidType {
		return "", false
	}
	core, exists := a.types["RoundingMode"]
	member, isMember := arg.(*ast.MemberExpression)
	if !exists || !sameConcreteType(modeType, core) || !isMember || member.Property == nil {
		a.addErrorAtTokenWithMetadata(expressionToken(arg), diagnostics.DurationConversionInvalid, durationConversionHelp,
			"the rounding argument of a duration conversion must be a RoundingMode case such as RoundingMode.Ceil, got %s", arg.String())
		return "", false
	}
	if _, ok := roundRat(big.NewRat(1, 2), member.Property.Value); !ok {
		return "", false
	}
	return member.Property.Value, true
}

// contextualDurationConversion accepts a time quantity where duration is
// expected (typed initialization, call argument) only when the conversion is
// proved exact and in range; it records the plan. A bare quantity never
// becomes a duration without such a context.
func (a *Analyzer) contextualDurationConversion(target, value Type, expr ast.Expression, record bool) bool {
	if expr == nil || !a.isDurationType(target) || !hasUnitSemantics(value) {
		return false
	}
	plan, _, ok := a.planDurationConversion(expr, value, "", false)
	if ok && record {
		a.durationConversions[expr] = plan
	}
	return ok
}

// durationConversionRejection explains why a contextual quantity cannot
// initialize a duration, for the ordinary initialization diagnostic.
func (a *Analyzer) durationConversionRejection(target, value Type, expr ast.Expression) string {
	if expr == nil || !a.isDurationType(target) || !hasUnitSemantics(value) {
		return ""
	}
	_, reason, ok := a.planDurationConversion(expr, value, "", false)
	if ok {
		return ""
	}
	return reason
}

const durationConversionHelp = "A time quantity converts to duration only when its exact nanosecond count is a whole number inside the signed 64-bit range: use whole nanoseconds, pass a RoundingMode (duration(q, RoundingMode.Ceil)), or check a runtime value with try duration(q)."
