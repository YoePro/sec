package sema

import (
	"math/big"

	"sec/internal/ast"
)

// inferQuantityLiteral types a numeric literal with a unit annotation such as
// `5<s>`. The unit resolves through the ordinary unit environment, never a
// fixed list. An expected unit-bearing numeric type with the same unit
// coordinates supplies the carrier; otherwise the unit's default carrier
// does (`<s>` is decimal<s> unless the unit declares another default). The
// literal must initialize that carrier exactly. A time quantity stays a
// unit-bearing quantity: it becomes a duration only through an explicit or
// contextual duration conversion.
//
// Rules:
//   - rules/types/units.md — "Quantity literals", "Unit-only type shorthand"
//   - rules/types/temporal.md — "Conversion of time quantities to duration"
func (a *Analyzer) inferQuantityLiteral(expr *ast.QuantityLiteral) (Type, expressionValue) {
	display := expressionValue{Display: expr.String()}
	unitType, ok := a.resolveUnitOnlyType(&ast.TypeReference{Token: expr.UnitToken, Unit: expr.Unit, UnitExpression: expr.UnitExpression, UnitOnly: true})
	if !ok {
		return Type{Kind: InvalidType}, display
	}
	literalType, _ := a.inferExpression(expr.Value)
	if literalType.Kind == InvalidType {
		return Type{Kind: InvalidType}, display
	}
	carrier := unitType
	if expected, has := a.expectedExpressionTypes[expr]; has && sameUnitCoordinates(expected, unitType) {
		carrier = expected
	}
	if !canInitialize(carrier, literalType, expr.Value) {
		a.addErrorAtToken(expr.Token, "quantity literal %s cannot be represented in its carrier %s", expr.String(), typeDisplayName(carrier))
		return Type{Kind: InvalidType}, display
	}
	return carrier, display
}

// sameUnitCoordinates reports a unit-bearing numeric type whose unit has the
// same dimension, compatible Kind and identical coordinates as unitType, so a
// quantity literal written in unitType's unit can take its carrier.
func sameUnitCoordinates(candidate, unitType Type) bool {
	if !isNumericType(candidate) || !hasUnitSemantics(candidate) || !candidate.Dimension.Equal(unitType.Dimension) {
		return false
	}
	to, from := effectiveUnitSemantics(candidate), effectiveUnitSemantics(unitType)
	if !unitKindCompatible(to, from) || to.Role != from.Role {
		return false
	}
	factor, offset, ok := unitConversionCoordinates(from, to)
	return ok && factor.Cmp(big.NewRat(1, 1)) == 0 && offset.Sign() == 0
}
