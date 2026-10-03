package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
	"sec/internal/lexer"
)

// validateParameterTypeShadowing rejects a function or method parameter whose
// name hides a source-defined type in the same module. Type declarations are
// collected before callable signatures, so this also catches forward types.
// Other visible namespaces remain part of the broader shadowing integration.
// Unit symbols live in their own namespace and are never shadowed by a
// parameter.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — §3 module declaration surface
//   - rules/foundations/names_scopes_visibility.md — §8 Shadowing
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — § 8
func (a *Analyzer) validateParameterTypeShadowing(parameter *ast.Identifier) {
	if parameter == nil || a.isUnitSymbol(parameter.Value) {
		return
	}
	typ, exists := a.types[parameter.Value]
	if !exists || typ.Module != a.currentModule {
		return
	}
	previous, exists := a.typeDefinitionTokens[parameter.Value]
	if !exists || !validDefinitionToken(previous) {
		return
	}
	a.addErrorAtTokenWithPreviousID(parameter.Token, previous, diagnostics.ParameterShadowsType,
		"parameter %s shadows visible type %s", parameter.Value, parameter.Value)
}

// reportLocalShadowsDeclaration rejects a local binding (variable, loop
// binding, pattern binding, or lambda parameter) that hides a visible
// generic parameter, same-module source type, or accessible free function.
// The binding is still declared so later uses do not cascade. Unit symbols
// occupy a separate namespace, so a local never shadows a unit.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — §2 "One declaration namespace per scope", §8 "Shadowing"
//   - rules/foundations/names_scopes_visibility.md — §20 "Diagnostics": "local declaration count shadows visible declaration count"
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — § 8
func (a *Analyzer) reportLocalShadowsDeclaration(name string, token lexer.Token) {
	if previous, exists := a.genericTypeDefinitions[name]; exists && validDefinitionToken(previous) {
		a.addErrorAtTokenWithPreviousID(token, previous, diagnostics.LocalShadowsDeclaration,
			"local declaration %s shadows visible generic parameter %s", name, name)
		return
	}
	if a.isUnitSymbol(name) {
		return
	}
	if typ, exists := a.types[name]; exists && typ.Module == a.currentModule {
		if previous, exists := a.typeDefinitionTokens[name]; exists && validDefinitionToken(previous) {
			a.addErrorAtTokenWithPreviousID(token, previous, diagnostics.LocalShadowsDeclaration,
				"local declaration %s shadows visible type %s", name, name)
			return
		}
	}
	for _, function := range a.accessibleFunctions(a.functions[name]) {
		if function.ImplTarget != "" || !validDefinitionToken(function.Token) {
			continue
		}
		a.addErrorAtTokenWithPreviousID(token, function.Token, diagnostics.LocalShadowsDeclaration,
			"local declaration %s shadows visible function %s", name, name)
		return
	}
}

// isUnitSymbol reports a declared unit symbol. Unit symbols occupy their own
// namespace and never conflict with, or are shadowed by, ordinary identifiers.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — §2 "One declaration namespace per scope"
//   - rules/types/units.md — "Unit names and compiler-known names"
//   - rules/corrections/applied/missing-decisions-md001-md009-correction-20261003.md — § 8
func (a *Analyzer) isUnitSymbol(name string) bool {
	_, ok := a.units[name]
	return ok
}
