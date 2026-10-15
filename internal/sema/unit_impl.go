package sema

import (
	"sec/internal/ast"
)

// unitFunctionKey is the function-table key of `<U>.Name`, and of `<U>` for
// the unit's constructor-style conversion function. The `<...>` spelling
// keeps unit functions apart from the functions of a same-spelled ordinary
// type (MD-040).
func unitFunctionKey(unit, name string) string {
	if name == "" {
		return "<" + unit + ">"
	}
	return "<" + unit + ">." + name
}

// registerUnitImplStatement registers the functions of `impl <U>`: each as
// `<U>.Name`, and a conversion function named like the unit additionally as
// `<U>`, the callee of `<U>(value)`. Metadata is analyzed separately. Other
// members have no meaning in a unit impl.
//
// Rules:
//   - rules/types/units.md — "Explicit unit conversion functions", "Unit names and compiler-known names"
//   - rules/declarations/impl.md — unit impl blocks (MD-040)
func (a *Analyzer) registerUnitImplStatement(stmt *ast.ImplStatement) {
	unit := stmt.Target.Name
	for _, member := range stmt.Members {
		switch member := member.(type) {
		case *ast.UnitMetadataDeclaration, *ast.InvalidStatement, *ast.InvalidMember:
		case *ast.FreeDeclaration:
			// MD-025.4: a unit describes measurement semantics, not an
			// independently destructible instance.
			a.addErrorAtTokenWithMetadata(member.Token, "",
				"Units express measurement semantics and have no instance to destroy; declare free on the nominal type that owns the resource.",
				"free is not permitted on a unit: impl <%s> cannot declare free", unit)
		case *ast.FunctionDeclaration:
			if member == nil || member.Name == nil {
				continue
			}
			if len(member.GenericParameters) > 0 {
				a.addErrorAtToken(member.Name.Token, "generic functions are not supported in impl <%s>", unit)
				continue
			}
			a.withImplTarget(unitFunctionKey(unit, ""), func() {
				a.registerFunctionDeclarationNamed(member, unitFunctionKey(unit, member.Name.Value))
				if a.isUnitConversionFunctionTarget(unit, member) && a.validateUnitConversionDimensions(unit, member) {
					a.registerFunctionDeclarationNamed(member, unitFunctionKey(unit, ""))
				}
			})
		default:
			a.addErrorAtToken(stmt.Target.Token, "impl <%s> may contain only unit metadata and functions", unit)
		}
	}
}

// analyzeUnitImplBody analyzes the function bodies of `impl <U>`.
func (a *Analyzer) analyzeUnitImplBody(stmt *ast.ImplStatement) {
	unit := stmt.Target.Name
	for _, member := range stmt.Members {
		fn, ok := member.(*ast.FunctionDeclaration)
		if !ok || fn == nil || fn.Name == nil || len(fn.GenericParameters) > 0 {
			continue
		}
		a.withImplTarget(unitFunctionKey(unit, ""), func() {
			a.analyzeFunctionBodyNamed(fn, unitFunctionKey(unit, fn.Name.Value))
		})
	}
}

// inferUnitMemberExpression types `<U>.Member` used as a value: the unit's
// declared `Symbol` and `LongName` text. A unit function is reached by
// calling it, `<U>.Name(...)`, through ordinary call resolution.
//
// Rules:
//   - rules/types/units.md — "Unit names and compiler-known names" (qualified unit member access, MD-040)
func (a *Analyzer) inferUnitMemberExpression(expr *ast.MemberExpression, reference *ast.UnitReferenceExpression) (Type, bool) {
	if expr.Property == nil {
		return Type{Kind: InvalidType}, false
	}
	unit, ok := a.units[reference.Unit]
	if !ok || reference.UnitExpression == nil || reference.UnitExpression.Kind != ast.UnitExpressionName {
		a.addErrorAtToken(reference.Token, "unknown unit %s", reference.String())
		return Type{Kind: InvalidType}, false
	}
	a.bindDefinition(reference.Token, unit.Token)
	switch expr.Property.Value {
	case "Symbol", "LongName":
		return Type{Name: "string", Kind: StringType}, true
	case "Kind", "Dimension", "Scale", "System", "BaseUnit", "Status", "Offset", "Origin", "LogBase", "LogFactor", "Reference":
		a.addErrorAtTokenWithMetadata(expr.Property.Token, "",
			"Only Symbol and LongName have a Sec 0.1 value type; the value types of the other unit metadata are undecided (missing-decisions.yaml).",
			"unit metadata %s.%s has no value type in Sec 0.1", reference.String(), expr.Property.Value)
		return Type{Kind: InvalidType}, false
	}
	if _, function := a.functions[unitFunctionKey(reference.Unit, expr.Property.Value)]; function {
		a.addErrorAtToken(expr.Property.Token, "unit function %s.%s must be called", reference.String(), expr.Property.Value)
		return Type{Kind: InvalidType}, false
	}
	a.addErrorAtToken(expr.Property.Token, "unit %s has no member %s", reference.String(), expr.Property.Value)
	return Type{Kind: InvalidType}, false
}
