package sema

import (
	"sec/internal/ast"
	"sec/internal/diagnostics"
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
