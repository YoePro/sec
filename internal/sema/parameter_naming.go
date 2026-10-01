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
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — §3 module declaration surface
//   - rules/foundations/names_scopes_visibility.md — §8 Shadowing
func (a *Analyzer) validateParameterTypeShadowing(parameter *ast.Identifier) {
	if parameter == nil {
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
// The binding is still declared so later uses do not cascade. Unit names are
// exempt until the unit namespace decision is recorded.
//
// Rules:
//   - rules/foundations/names_scopes_visibility.md — §2 "One declaration namespace per scope", §8 "Shadowing"
//   - rules/foundations/names_scopes_visibility.md — §20 "Diagnostics": "local declaration count shadows visible declaration count"
func (a *Analyzer) reportLocalShadowsDeclaration(name string, token lexer.Token) {
	if previous, exists := a.genericTypeDefinitions[name]; exists && validDefinitionToken(previous) {
		a.addErrorAtTokenWithPreviousID(token, previous, diagnostics.LocalShadowsDeclaration,
			"local declaration %s shadows visible generic parameter %s", name, name)
		return
	}
	// Whether unit declarations occupy the shared declaration namespace is not
	// yet decided (missing-decisions.yaml MD-007), so unit names are exempt.
	if _, isUnit := a.units[name]; isUnit {
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
