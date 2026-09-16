package sema

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"sec/internal/ast"
	"sec/internal/diagnostics"
)

// validateGenericTypeParameterNames enforces uppercase type names after the
// visibility prefix. Parameters stay available for recovery; appendError
// deduplicates diagnostics when a declaration participates in multiple passes.
// Rules: rules/foundations/names_scopes_visibility.md — §12 Visibility prefixes,
// §13 Type naming, §15 Generic parameters;
// rules/declarations/generics.md — §2 Generic parameter syntax.
func (a *Analyzer) validateGenericTypeParameterNames(parameters []*ast.GenericParameter) {
	for _, parameter := range parameters {
		if parameter == nil || parameter.Name == nil {
			continue
		}
		name := parameter.Name.Value
		first, _ := utf8.DecodeRuneInString(strings.TrimLeft(name, "_"))
		if unicode.IsUpper(first) {
			continue
		}
		a.addErrorAtTokenWithMetadata(parameter.Name.Token, diagnostics.InvalidGenericParameterName,
			"use an uppercase initial after any visibility prefix and update references to this type parameter",
			"generic type parameter %s must begin with an uppercase letter after any visibility prefix", name)
	}
}

// validateGenericParameterTypeShadowing rejects a generic parameter that hides
// a source-defined type in the same module. The check runs when entering the
// generic scope, after the complete module type surface has been registered.
// Other visibility sources are left to the broader namespace audit.
//
// Rules: rules/foundations/names_scopes_visibility.md — §8 Shadowing and
// §15 Generic parameters (visible type prohibition).
func (a *Analyzer) validateGenericParameterTypeShadowing(parameters []*ast.GenericParameter) {
	module := a.currentModule
	if a.currentImplTarget != "" {
		// Nested impl members are analyzed with their owner set, but without
		// necessarily setting currentModule on this pass.
		if target, exists := a.types[a.currentImplTarget]; exists {
			module = target.Module
		}
	}
	for _, parameter := range parameters {
		if parameter == nil || parameter.Name == nil {
			continue
		}
		name := parameter.Name.Value
		typ, exists := a.types[name]
		if !exists || typ.Module != module {
			continue
		}
		previous, exists := a.typeDefinitionTokens[name]
		if !exists || !validDefinitionToken(previous) {
			continue
		}
		a.addErrorAtTokenWithPreviousID(parameter.Name.Token, previous,
			diagnostics.GenericParameterShadowsType,
			"generic parameter %s shadows visible type %s", name, name)
	}
}
