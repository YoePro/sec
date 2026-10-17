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
	typ, ok := a.ResolvedTypeOf(expr)
	if !ok || typ.Kind != EnumType {
		return ResolvedEnumCase{}, false
	}
	value, ok := typ.EnumConsts[expr.Property.Value]
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
// — §§ 19–20, 24 (selected calls retain their callable identity).
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
		return ResolvedEnumConversion{Kind: ResolvedEnumToInteger, EnumType: operand, IntegerType: result}, true
	}
	if operand.Kind == EnumType && result.Kind == ResultType && len(result.TypeArgs) == 2 && isIntegerType(result.TypeArgs[0]) {
		return ResolvedEnumConversion{Kind: ResolvedEnumToInteger, EnumType: operand, IntegerType: result.TypeArgs[0], Fallible: true}, true
	}
	return ResolvedEnumConversion{}, false
}
