package sema

import (
	"math/big"
	"sec/internal/ast"
	"sec/internal/sema/constant"
	"sort"
)

// UnitConversionSuggestion is a compiler-derived explicit conversion candidate
// for a rejected quantity value. Tooling must validate the complete replacement
// to check overload selection and contracts. The span uses one-based scalar
// columns and an exclusive end; Replacement is the complete converted
// expression.
type UnitConversionSuggestion struct {
	Kind        string
	File        string
	Line        int
	Column      int
	EndLine     int
	EndColumn   int
	Target      string
	Replacement string
}

// UnitConversionSuggestions returns a snapshot of compiler-derived conversions
// recorded during analysis, for source-validated tooling code actions.
func (a *Analyzer) UnitConversionSuggestions() []UnitConversionSuggestion {
	return append([]UnitConversionSuggestion(nil), a.unitConversionSuggestions...)
}

// unitConversionSuggestion proves that wrapping a rejected value in the
// target's named unit, `Target(value)`, yields exactly the expected type: the
// target has a named unit, the explicit conversion is valid, and the carrier
// is unchanged so the conversion introduces no hidden carrier change. Only a
// binding or a field chain is offered, so the source span is exact. No factor,
// exchange rate, or rounding policy is ever invented.
//
// Rules:
//   - rules/tooling/lsp.md — "Unit actions" (compiler-proven fixed conversion; no invented exchange rate or rounding)
//   - rules/types/units.md — explicit conversion functions, "LSP requirements"
func (a *Analyzer) unitConversionSuggestion(expected Type, actual Type, expr ast.Expression) (UnitConversionSuggestion, bool) {
	target := effectiveUnitSemantics(expected)
	if target.Named == "" || !hasUnitSemantics(actual) || !explicitUnitConversionProven(expected, actual) {
		return UnitConversionSuggestion{}, false
	}
	if effectiveUnitSemantics(actual).Named == target.Named || expected.Kind != actual.Kind || unitCarrierName(expected) != unitCarrierName(actual) {
		return UnitConversionSuggestion{}, false
	}
	if _, ok := a.units[target.Named]; !ok {
		return UnitConversionSuggestion{}, false
	}
	spelling, ok := simpleValueSpelling(expr)
	if !ok {
		return UnitConversionSuggestion{}, false
	}
	start := expressionToken(expr)
	endLine, endColumn := expressionEndToken(expr).EndPosition()
	return UnitConversionSuggestion{
		File: start.File, Line: start.Line, Column: start.Column, EndLine: endLine, EndColumn: endColumn,
		Target:      target.Named,
		Replacement: target.Named + "(" + spelling + ")",
	}, true
}

// unitCarrierName is the numeric carrier of a quantity type without its unit.
func unitCarrierName(typ Type) string {
	return string(typ.Kind) + ":" + numericCarrierName(typ)
}

// simpleValueSpelling spells a binding or a chain of member reads.
func simpleValueSpelling(expr ast.Expression) (string, bool) {
	switch expr := expr.(type) {
	case *ast.Identifier:
		return expr.Value, true
	case *ast.MemberExpression:
		base, ok := simpleValueSpelling(expr.Object)
		if !ok || expr.Property == nil {
			return "", false
		}
		return base + "." + expr.Property.Value, true
	}
	return "", false
}

// unitConversionAlternatives records only declared callable relations, actual
// in-scope factors, or a proved exact carrier change at a rejected value site.
// Final source replacement validation remains required for overloads/contracts.
// Rules: rules/types/units.md — Explicit unit conversion functions, Constructor-style
// unit conversion, Factor-provided conversions, No hidden precision loss;
// rules/tooling/lsp.md — Unit actions, Safe fixes.
func (a *Analyzer) unitConversionAlternatives(expected, actual Type, expr ast.Expression) []UnitConversionSuggestion {
	out := []UnitConversionSuggestion{}
	if fixed, ok := a.unitConversionSuggestion(expected, actual, expr); ok {
		out = append(out, fixed)
	}
	to, from := effectiveUnitSemantics(expected), effectiveUnitSemantics(actual)
	spelling, ok := simpleValueSpelling(expr)
	if !ok || !hasUnitSemantics(actual) {
		return out
	}
	start := expressionToken(expr)
	endLine, endColumn := expressionEndToken(expr).EndPosition()
	add := func(kind, replacement string) {
		for _, prior := range out {
			if prior.Replacement == replacement {
				if kind == "declared-function" {
					for i := range out {
						if out[i].Replacement == replacement {
							out[i].Kind = kind
						}
					}
				}
				return
			}
		}
		out = append(out, UnitConversionSuggestion{Kind: kind, File: start.File, Line: start.Line, Column: start.Column, EndLine: endLine, EndColumn: endColumn, Target: to.Named, Replacement: replacement})
	}
	for _, fn := range a.accessibleFunctions(a.functions[to.Named]) {
		if fn.ImplTarget != to.Named || fn.Unsafe || len(fn.Parameters) != 1 || len(fn.GenericParameters) != 0 {
			continue
		}
		parameter := fn.Parameters[0].Type
		if sameNumericCarrier(parameter, actual) && effectiveUnitSemantics(parameter).Named == from.Named && canInitialize(expected, fn.ReturnType, nil) {
			add("declared-function", to.Named+"("+spelling+")")
		}
	}
	if to.Named != "" && sameNumericCarrier(expected, actual) && isLinearRatioSemantics(to) && isLinearRatioSemantics(from) {
		names := []string{}
		for name := range a.symbols {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			symbol := a.symbols[name]
			factor := symbol.Type
			if symbol.ImplicitMember || !isNumericType(factor) || !hasUnitSemantics(factor) || !sameNumericCarrier(factor, actual) || !isLinearRatioSemantics(effectiveUnitSemantics(factor)) || !factor.Dimension.Equal(expected.Dimension.Div(actual.Dimension)) {
				continue
			}
			add("in-scope-factor", to.Named+"("+spelling+", "+name+")")
		}
	}
	if !sameNumericCarrier(expected, actual) && a.exactUnitCarrierChange(expected, actual, expr) {
		cast := numericCarrierName(expected) + "(" + spelling + ")"
		if to.Named != from.Named {
			if to.Named == "" || !explicitUnitConversionProven(expected, actual) {
				return out
			}
			cast = to.Named + "(" + cast + ")"
		}
		add("exact-carrier", cast)
	}
	return out
}

// exactUnitCarrierChange proves lossless conversion of every admitted integer
// or binary-floating source value, or a single exact compile-time scalar.
// It never recommends a potentially lossy runtime cast or a rounding policy.
// Rules: rules/types/units.md — Numeric carrier conversion, No hidden precision loss;
// rules/types/types.md — Explicit conversions, Binary floating-point types;
// rules/memory/layout.md — scalar representations.
func (a *Analyzer) exactUnitCarrierChange(target, source Type, expr ast.Expression) bool {
	if !isNumericType(target) || !isNumericType(source) {
		return false
	}
	if source.Kind == FloatType && target.Kind == FloatType && source.FloatBits == 32 && target.FloatBits == 64 {
		return true
	}
	if source.Kind == IntType || source.Kind == UintType {
		low, high, ok := a.integerInterval(expr)
		if !ok {
			return false
		}
		switch target.Kind {
		case IntType, UintType:
			min, max, bounded := integerTypeInterval(target)
			return bounded && low.Cmp(min) >= 0 && high.Cmp(max) <= 0
		case FloatType:
			precision := 53
			if target.FloatBits == 32 {
				precision = 24
			} else if target.FloatBits != 64 {
				return false
			}
			limit := new(big.Int).Lsh(big.NewInt(1), uint(precision))
			return new(big.Int).Abs(low).Cmp(limit) <= 0 && new(big.Int).Abs(high).Cmp(limit) <= 0
		case DecimalType:
			bits := a.compileTimeDecimalWidth(target)
			limit := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
			return low.Cmp(new(big.Int).Neg(limit)) >= 0 && high.Cmp(new(big.Int).Sub(limit, big.NewInt(1))) <= 0
		}
	}
	value, outcome := a.semanticCompileTimeConstant(expr)
	if outcome != compileTimeEvaluated {
		return false
	}
	exact := value.Exact
	if exact == nil && value.Integer != nil {
		exact = new(big.Rat).SetInt(value.Integer)
	}
	if exact == nil {
		return false
	}
	switch target.Kind {
	case IntType, UintType:
		if !exact.IsInt() {
			return false
		}
		min, max, bounded := integerTypeInterval(target)
		return bounded && exact.Num().Cmp(min) >= 0 && exact.Num().Cmp(max) <= 0
	case DecimalType:
		_, ok := constant.DecimalText(exact, a.compileTimeDecimalWidth(target))
		return ok
	case FloatType:
		f, _ := exact.Float64()
		rounded, ok := constant.FloatExact(f, target.FloatBits)
		return ok && rounded.Cmp(exact) == 0
	}
	return false
}
