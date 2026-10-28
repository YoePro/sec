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
		case *ast.UnitMetadataDeclaration:
		case *ast.InvalidStatement:
			if member.Message != "" && !parserReported(member.Recovery) {
				a.addErrorAtToken(member.Token, "%s", member.Message)
			}
		case *ast.InvalidMember:
			// A field line such as `Name:` in impl <U> (MD-062 § 6(3)).
			if member.Message != "" && !parserReported(member.Recovery) {
				a.addErrorAtToken(member.Token, "%s", member.Message)
			}
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

// inferUnitMemberExpression types `<U>.Member` used as a value: one of the
// 16 typed metadata members of a named unit. A unit function is reached by
// calling it, `<U>.Name(...)`, through ordinary call resolution.
//
// Rules:
//   - rules/types/units.md — "Unit names and compiler-known names" (qualified unit member access, MD-040)
//   - rules/types/units.md — "Unit metadata value types" (1), (2), (11) (MD-062)
func (a *Analyzer) inferUnitMemberExpression(expr *ast.MemberExpression, reference *ast.UnitReferenceExpression) (Type, bool) {
	if expr.Property == nil {
		return Type{Kind: InvalidType}, false
	}
	if reference.UnitExpression != nil && reference.UnitExpression.Kind != ast.UnitExpressionName && isUnitMetadataMember(expr.Property.Value) {
		a.addErrorAtTokenWithMetadata(expr.Property.Token, "",
			"unit metadata belongs to named units; declare a named unit with this structure to read its metadata",
			"%s is a structural unit expression and has no %s metadata", reference.String(), expr.Property.Value)
		return Type{Kind: InvalidType}, false
	}
	name := reference.Unit
	if reference.UnitExpression != nil && reference.UnitExpression.Kind == ast.UnitExpressionName {
		name = reference.UnitExpression.Name
	}
	unit, ok := a.units[name]
	if !ok || reference.UnitExpression == nil || reference.UnitExpression.Kind != ast.UnitExpressionName {
		a.addErrorAtToken(reference.Token, "unknown unit %s", reference.String())
		return Type{Kind: InvalidType}, false
	}
	// Bind the real unit identifier, never the '<' delimiter or import alias.
	// Rules: rules/tooling/lsp.md — "Navigation and references", "Rename".
	use := reference.UnitExpression.NameToken
	if use.Type == "" {
		use = reference.UnitExpression.Token
	}
	a.bindDefinition(use, unit.Token)
	if typ, metadata := a.unitMetadataMemberType(reference, unit, expr.Property); metadata {
		if typ.Kind == InvalidType {
			return typ, false
		}
		return a.recordUnitMetadataRead(expr, reference, unit, typ)
	}
	if _, function := a.functions[unitFunctionKey(name, expr.Property.Value)]; function {
		a.addErrorAtToken(expr.Property.Token, "unit function %s.%s must be called", reference.String(), expr.Property.Value)
		return Type{Kind: InvalidType}, false
	}
	a.addErrorAtToken(expr.Property.Token, "unit %s has no member %s", reference.String(), expr.Property.Value)
	return Type{Kind: InvalidType}, false
}
