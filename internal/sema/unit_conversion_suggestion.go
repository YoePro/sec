package sema

import (
	"sec/internal/ast"
)

// UnitConversionSuggestion is a compiler-proven explicit unit conversion for
// a value that was rejected where another unit of the same quantity was
// required. The span covers the value expression with one-based scalar
// columns and an exclusive end; Replacement is the complete converted
// expression.
type UnitConversionSuggestion struct {
	File        string
	Line        int
	Column      int
	EndLine     int
	EndColumn   int
	Target      string
	Replacement string
}

// UnitConversionSuggestions returns the proven conversions recorded while
// analyzing, for tooling code actions.
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
	if typ.Underlying != "" && typ.Named {
		return typ.Underlying
	}
	return string(typ.Kind) + ":" + typ.Name
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
