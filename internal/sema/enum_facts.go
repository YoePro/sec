package sema

import (
	"math/big"
	"sec/internal/ast"
)

// ResolvedEnumCaseOf returns the exact declared case selected by a successfully
// analyzed member expression. Alias cases retain distinct ordinals.
// Rules: rules/declarations/enums.md — §§ 4, 7;
// rules/compiler/semantic_ir.md — resolved enum case identities.
func (a *Analyzer) ResolvedEnumCaseOf(expr *ast.MemberExpression) (ResolvedEnumCase, bool) {
	if a == nil || expr == nil || expr.Property == nil {
		return ResolvedEnumCase{}, false
	}
	// `<U>.Kind` is a unit metadata read even when its enum has a member of
	// the same spelling (MD-062 § 11(2)).
	if _, unit := expr.Object.(*ast.UnitReferenceExpression); unit {
		return ResolvedEnumCase{}, false
	}
	typ, ok := a.ResolvedTypeOf(expr)
	if !ok || typ.Kind != EnumType {
		return ResolvedEnumCase{}, false
	}
	return enumCaseNamed(typ, expr.Property.Value)
}

// enumCaseNamed resolves the declared case of an enum type by member name.
// Rules: rules/declarations/enums.md — §§ 4, 7.
func enumCaseNamed(typ Type, member string) (ResolvedEnumCase, bool) {
	value, ok := typ.EnumConsts[member]
	if !ok {
		return ResolvedEnumCase{}, false
	}
	for ordinal, name := range typ.EnumValues {
		if name == value.Name {
			resolved := ResolvedEnumCase{EnumType: typ, Name: value.Name, Ordinal: uint32(ordinal), Token: value.Token}
			if value.Value != nil {
				resolved.Value = new(big.Int).Set(value.Value)
			}
			if value.StringValue != nil {
				stringValue := *value.StringValue
				resolved.StringValue = &stringValue
			}
			if resolved.Value == nil && resolved.StringValue == nil {
				return ResolvedEnumCase{}, false
			}
			return resolved, true
		}
	}
	return ResolvedEnumCase{}, false
}

// ResolvedEnumConversionOf classifies only explicit conversions accepted by
// Sema. It does not infer a conversion from callee spelling alone.
// Rules: rules/declarations/enums.md — § 12; rules/declarations/functions.md
// — §§ 19–20, 24 (selected calls retain their callable identity);
// rules/types/types.md — "Exact integer conversion proof" (known constants).
func (a *Analyzer) ResolvedEnumConversionOf(call *ast.CallExpression) (ResolvedEnumConversion, bool) {
	if a == nil || call == nil || len(call.Arguments) != 1 {
		return ResolvedEnumConversion{}, false
	}
	if _, selected := a.ResolvedCallTarget(call); selected {
		return ResolvedEnumConversion{}, false
	}
	if callee, resolved := a.ResolvedTypeOf(call.Callee); resolved && callee.Kind == FunctionType {
		return ResolvedEnumConversion{}, false
	}
	result, resultOK := a.ResolvedTypeOf(call)
	operand, operandOK := a.ResolvedTypeOf(call.Arguments[0])
	if !resultOK || !operandOK {
		return ResolvedEnumConversion{}, false
	}
	if result.Kind == EnumType && isIntegerType(operand) {
		return ResolvedEnumConversion{Kind: ResolvedIntegerToEnum, EnumType: result, IntegerType: operand}, true
	}
	if operand.Kind == EnumType && isIntegerType(result) {
		conversion := ResolvedEnumConversion{Kind: ResolvedEnumToInteger, EnumType: operand, IntegerType: result}
		if isBuiltinConversionInteger(result) {
			// Keep the same known-member evidence that admitted the source
			// conversion. The complete enum domain may be wider than this
			// constant, so recomputing only a domain proof loses exactness.
			constant, _ := enumConstantValue(operand, call.Arguments[0])
			conversion.Proof = a.integerConversionProof(result, operand, constant)
		}
		return conversion, true
	}
	if operand.Kind == EnumType && result.Kind == ResultType && len(result.TypeArgs) == 2 && isIntegerType(result.TypeArgs[0]) {
		return ResolvedEnumConversion{Kind: ResolvedEnumToInteger, EnumType: operand, IntegerType: result.TypeArgs[0], Fallible: true}, true
	}
	return ResolvedEnumConversion{}, false
}
