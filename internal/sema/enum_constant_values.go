package sema

import (
	"math/big"
	"sec/internal/ast"
)

// recordEnumConstant preserves exact values established during expression
// analysis. Only validated integer-to-enum constructors, declared cases and
// immutable bindings supply evidence; ordinary calls and runtime fields do not.
// Rules: rules/declarations/enums.md — §§12.1–12.3;
// rules/types/types.md — Exact integer conversion proof (MD-104).
func (a *Analyzer) recordEnumConstant(expr ast.Expression, typ Type) {
	delete(a.resolvedEnumConstants, expr)
	if typ.Kind != EnumType {
		return
	}
	var value *big.Int
	switch expr := expr.(type) {
	case *ast.MemberExpression:
		if resolved, ok := a.ResolvedEnumCaseOf(expr); ok {
			value = resolved.Value
		}
	case *ast.Identifier:
		if symbol, ok := a.symbols.lookup(expr.Value); ok && !symbol.Mutable && !symbol.TransientConstant && sameConcreteType(symbol.Type, typ) {
			value = symbol.enumConstant
		}
	case *ast.CallExpression:
		if conversion, ok := a.ResolvedEnumConversionOf(expr); ok && conversion.Kind == ResolvedIntegerToEnum {
			value, _ = a.constantConditionIntegerValue(expr.Arguments[0])
		}
	case *ast.ConversionExpression:
		if source, ok := a.ResolvedTypeOf(expr.Value); ok && isIntegerType(source) {
			value, _ = a.constantConditionIntegerValue(expr.Value)
		}
	}
	if value != nil {
		if a.resolvedEnumConstants == nil {
			a.resolvedEnumConstants = map[ast.Expression]*big.Int{}
		}
		a.resolvedEnumConstants[expr] = new(big.Int).Set(value)
	}
}

// setEnumBindingConstant retains an immutable binding's established value.
// Mutable/transient inputs never become lasting compile-time evidence, and
// enum constants stay separate from integer bindings to preserve nominal typing.
// Rules: rules/declarations/enums.md — §§4,12.1;
// rules/compiler/compile_time_evaluation.md — §10 (ordinary immutable bindings).
func (a *Analyzer) setEnumBindingConstant(symbol *Symbol, expr ast.Expression) {
	symbol.enumConstant = nil
	if symbol.Type.Kind != EnumType || symbol.Mutable || symbol.TransientConstant {
		return
	}
	if value, ok := a.enumConstantValue(symbol.Type, expr); ok {
		symbol.enumConstant = new(big.Int).Set(value)
	}
}
