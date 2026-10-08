package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/sema/unitexpr"
)

// validateConcreteUnitParameters keeps generic type parameters out of unit
// annotations while preserving the independent unit-symbol namespace.
// Rules: rules/types/units.md — Future unit-polymorphic generics;
// rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — §8.
func (a *Analyzer) validateConcreteUnitParameters(ref *ast.TypeReference) bool {
	factor := unitexpr.FirstName(ref.UnitExpression, func(name string) bool {
		_, parameter := a.genericTypes[name]
		_, declaredUnit := a.units[name]
		return parameter && !declaredUnit
	})
	if factor == nil {
		return true
	}
	a.addErrorAtTokenWithMetadata(factor.Token, diagnostics.UnitPolymorphismReserved,
		"Use a concrete declared unit; Sec 0.1 generic parameters represent types, not units.",
		"generic type parameter %s cannot be used as a unit; unit-polymorphic generics are reserved for a future language revision", factor.Name)
	return false
}
