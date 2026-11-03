package sema

import (
	"math/big"
	"sec/internal/ast"
)

// ResolvedEnumCaseOf returns the exact declared case selected by a successfully
// analyzed member expression from its retained selected owner. Runtime fields
// and getters never become cases by spelling. Alias cases retain distinct ordinals.
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
	owner, selected := a.resolvedEnumCaseOwners[expr]
	if !selected || !sameConcreteType(owner, typ) {
		return ResolvedEnumCase{}, false
	}
	return enumCaseNamed(owner, expr.Property.Value)
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
// Sema. Enum-to-integer decisions are retained during inference, never
// recomputed from mutable query-time state or callee spelling.
// Rules: rules/declarations/enums.md — § 12; rules/declarations/functions.md
// — §§ 19–20, 24 (selected calls retain their callable identity);
// rules/types/types.md — "Exact integer conversion proof" (known constants).
func (a *Analyzer) ResolvedEnumConversionOf(call *ast.CallExpression) (ResolvedEnumConversion, bool) {
	if a == nil || call == nil {
		return ResolvedEnumConversion{}, false
	}
	if fact, recorded := a.resolvedEnumConversions[call]; recorded {
		return cloneEnumConversion(fact), true
	}
	if len(call.Arguments) != 1 {
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
		return cloneEnumConversion(ResolvedEnumConversion{Kind: ResolvedIntegerToEnum, EnumType: result, IntegerType: operand}), true
	}

	return ResolvedEnumConversion{}, false
}
