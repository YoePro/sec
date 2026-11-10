package sema

import (
	"sec/internal/ast"
	"sec/internal/lexer"
)

// withGenericTypeParameters enters a nested generic scope while preserving
// outer declarations and restoring their types and definition origins on exit.
// Reopening an impl owner's parameters preserves its declared constraints.
// Rules: rules/declarations/generics.md — §§8–9,15;
// rules/foundations/names_scopes_visibility.md — §§8,15,17.
func (a *Analyzer) withGenericTypeParameters(parameters []*ast.GenericParameter, visit func()) {
	a.validateGenericParameterTypeShadowing(parameters)
	previous := a.genericTypes
	previousDefinitions := a.genericTypeDefinitions
	current := map[string]Type{}
	currentDefinitions := map[string]lexer.Token{}
	for name, typ := range previous {
		current[name] = typ
	}
	for name, token := range previousDefinitions {
		currentDefinitions[name] = token
	}
	for _, param := range parameters {
		if param == nil || param.Name == nil {
			continue
		}
		kind := GenericType
		if param.TypePack {
			kind = GenericTypePackType
		}
		current[param.Name.Value] = Type{
			Name: param.Name.Value,
			Kind: kind,
		}
		currentDefinitions[param.Name.Value] = param.Name.Token
		a.recordDefinition(param.Name.Token)
	}
	a.genericTypes = current
	a.genericTypeDefinitions = currentDefinitions
	for _, param := range parameters {
		if param == nil || param.Name == nil {
			continue
		}
		a.genericTypes[param.Name.Value] = a.constrainedGenericParameterType(param)
		a.recordGenericParameterFact(a.genericTypes[param.Name.Value], param.Name.Token, a.implGenericParameterTargets[param])
	}
	defer func() {
		a.genericTypes = previous
		a.genericTypeDefinitions = previousDefinitions
	}()
	visit()
}

// analyzeFunctionBodyNamed validates generic headers even if heterogeneous
// body expansion reaches an implementation gate. Unsupported bodies cannot
// hide a conflicting declaration; supported bodies use the same scoped types.
// Rules: rules/declarations/generics.md — §§9,16A;
// rules/foundations/names_scopes_visibility.md — §§8,15.
func (a *Analyzer) analyzeFunctionBodyNamed(fn *ast.FunctionDeclaration, name string) {
	if a.rejectUnavailableTypePackExpansion(fn) {
		a.validateGenericParameterTypeShadowing(fn.GenericParameters)
		return
	}
	if len(fn.GenericParameters) > 0 {
		a.withGenericTypeParameters(fn.GenericParameters, func() {
			a.analyzeFunctionBodyInScope(fn, name)
		})
		return
	}
	a.analyzeFunctionBodyInScope(fn, name)
}
